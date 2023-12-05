// Copyright 2023 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package main

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/canonical/sqlair"
	"github.com/google/uuid"
	"github.com/juju/collections/transform"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"gopkg.in/tomb.v2"
)

type ModelOperation func(Model, *sqlair.TX) error
type ModelsOperation func([]Model) error

var (
	timeBucketSplits = []float64{
		0.001,
		0.01,
		0.02,
		0.03,
		0.04,
		0.05,
		0.06,
		0.07,
		0.08,
		0.09,
		0.1,
		1.0,
		10.0,
	}
)

func SliceToPlaceholder[T any](in []T) string {
	return strings.Join(transform.Slice(in, func(item T) string { return "?" }), ",")
}

var seedModelAgent = sqlair.MustPrepare("INSERT INTO agent VALUES ($M.uuid, $Model.name, $M.inactive)", sqlair.M{}, Model{})

func seedModelAgents(numAgents int) ModelOperation {
	return func(model Model, tx *sqlair.TX) error {
		fmt.Println("Seeding agents")

		for i := 0; i < numAgents; i++ {
			uuid, err := uuid.NewUUID()
			if err != nil {
				return err
			}
			err = tx.Query(nil, seedModelAgent, sqlair.M{"uuid": uuid.String(), "inactive": "inactive"}, model).Run()
			if err != nil {
				return err
			}
		}
		return nil
	}
}

var selectUUID = sqlair.MustPrepare(`SELECT &M.uuid
FROM agent
WHERE model_name = $Model.name
ORDER BY RANDOM()
LIMIT $M.agentUpdates
`, sqlair.M{}, Model{})

func updateModelAgentStatus(agentUpdates int, status string) ModelOperation {
	return func(model Model, tx *sqlair.TX) error {
		fmt.Println("Updating agent status")

		ms := []sqlair.M{}
		err := tx.Query(nil, selectUUID, model, sqlair.M{"agentUpdates": agentUpdates}).GetAll(&ms)
		if err != nil {
			return err
		}

		createTable := sqlair.MustPrepare("CREATE TEMPORARY TABLE temp_agent_uuids ( uuid INT )")
		err = tx.Query(nil, createTable).Run()
		if err != nil {
			return nil
		}

		insertUUID := sqlair.MustPrepare("INSERT INTO temp_agent_uuids VALUES ($M.uuid)", sqlair.M{})
		for _, m := range ms {
			// INSERT m["uuid"] into temp table.
			err = tx.Query(nil, insertUUID, m).Run()
			if err != nil {
				return nil
			}
		}

		updateStatus := sqlair.MustPrepare("UPDATE agent SET status = $M.status WHERE uuid IN (SELECT uuid FROM temp_agent_uuids)", sqlair.M{})
		err = tx.Query(nil, updateStatus, sqlair.M{"status": status}).Run()
		if err != nil {
			return err
		}

		dropTable := sqlair.MustPrepare("DROP TABLE temp.temp_agent_uuids")
		err = tx.Query(nil, dropTable).Run()
		if err != nil {
			return err
		}

		return err
	}
}

var insertAgentStrings = sqlair.MustPrepare("INSERT INTO agent_events VALUES ($M.uuid, $M.event)", sqlair.M{})

func generateAgentEvents(agents int) ModelOperation {
	return func(model Model, tx *sqlair.TX) error {
		fmt.Println("Generating agent events")

		ms := []sqlair.M{}
		err := tx.Query(nil, selectUUID, sqlair.M{"agentUpdates": agents}, model).GetAll(&ms)
		if err != nil {
			return err
		}

		for _, m := range ms {
			m["event"] = "event"
			err = tx.Query(nil, insertAgentStrings, m).Run()
			if err != nil {
				return err
			}
		}

		return err
	}
}

func cullAgentEvents(maxEvents int) ModelOperation {
	return func(model Model, tx *sqlair.TX) error {
		fmt.Println("Culling agent events")
		// delete from agent_events where agent_uuid in (select agent_uuid from agent_events group by agent_uuid having count(*) > 1
		cullAgents := sqlair.MustPrepare("DELETE FROM agent_events WHERE agent_uuid IN (SELECT agent_uuid from agent_events INNER JOIN agent ON agent.uuid = agent_events.agent_uuid WHERE agent.model_name = $Model.name GROUP BY agent_uuid HAVING COUNT(*) > $M.maxEvents)", Model{}, sqlair.M{})
		err := tx.Query(nil, cullAgents, model, sqlair.M{"maxEvents": maxEvents}).Run()
		return err
	}
}

func agentModelCount(gaugeVec *prometheus.GaugeVec) ModelOperation {
	return func(model Model, tx *sqlair.TX) error {
		fmt.Println("Agent model count")

		getCount := sqlair.MustPrepare(`
SELECT &M.c FROM (
SELECT count(*) AS c
FROM agent
WHERE model_name = $Model.name)
`, sqlair.M{}, Model{})
		m := sqlair.M{}
		err := tx.Query(nil, getCount, model).Get(m)
		if err != nil {
			return err
		}

		count := m["c"].(int64)

		gauge, err := gaugeVec.GetMetricWith(prometheus.Labels{
			"model": model.Name,
		})
		if err != nil {
			return err
		}

		gauge.Set(float64(count))
		return nil
	}
}

func agentEventModelCount(gaugeVec *prometheus.GaugeVec) ModelOperation {
	return func(model Model, tx *sqlair.TX) error {
		fmt.Println("Agent event model count")

		eventModelCount := sqlair.MustPrepare(`
SELECT &M.c FROM (
SELECT count(*) AS c
FROM agent_events
INNER JOIN agent ON agent.uuid = agent_events.agent_uuid
WHERE agent.model_name = $Model.name)
`, Model{}, sqlair.M{})

		m := sqlair.M{}
		err := tx.Query(nil, eventModelCount, model).Get(m)
		if err != nil {
			return err
		}

		count := m["c"].(int64)

		gauge, err := gaugeVec.GetMetricWith(prometheus.Labels{
			"model": model.Name,
		})
		if err != nil {
			return err
		}

		gauge.Set(float64(count))
		return nil
	}
}

func runModelOpTx(
	op ModelOperation,
	model Model,
	obs prometheus.Observer,
) error {
	ctx := context.Background()
	timer := prometheus.NewTimer(obs)
	defer timer.ObserveDuration()
	return model.TxRunner(ctx, func(_ context.Context, tx *sqlair.TX) error {
		return op(model, tx)
	})
}

func RunModelOperation(
	t *tomb.Tomb,
	opName string,
	freq time.Duration,
	op ModelOperation,
	model Model,
) {
	t.Go(func() error {
		opHistogram := promauto.NewHistogram(prometheus.HistogramOpts{
			Name: "model_operation_time",
			ConstLabels: prometheus.Labels{
				"model":     model.Name,
				"operation": opName,
			},
			Buckets: timeBucketSplits,
		})
		opErrCount := promauto.NewCounter(prometheus.CounterOpts{
			Name: "model_operation_errors",
			ConstLabels: prometheus.Labels{
				"model":     model.Name,
				"operation": opName,
			},
		})

		if freq == time.Duration(0) {
			if err := runModelOpTx(op, model, opHistogram); err != nil {
				opErrCount.Inc()
				fmt.Printf("model operation %s died for model %s: %v\n", opName, model.Name, err)
			}
			return nil
		}

		initalDelay := time.Duration(rand.Int63n(int64(freq)))
		time.Sleep(initalDelay)

		ticker := time.NewTicker(freq)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := runModelOpTx(op, model, opHistogram); err != nil {
					opErrCount.Inc()
					fmt.Printf("model operation %s died for model %s: %v\n", opName, model.Name, err)
				}
			case <-t.Dying():
				return nil
			}
		}
	})
}

func RunModelsOperation(
	t *tomb.Tomb,
	opName string,
	freq time.Duration,
	op ModelsOperation,
	models []Model,
) {
	t.Go(func() error {
		ticker := time.NewTicker(freq)
		defer ticker.Stop()
		opHistogram := promauto.NewHistogram(prometheus.HistogramOpts{
			Name: "operation_times",
			ConstLabels: prometheus.Labels{
				"operation": opName,
			},
			Buckets: timeBucketSplits,
		})

		for {
			select {
			case <-ticker.C:
				timer := prometheus.NewTimer(opHistogram)
				if err := op(models); err != nil {
					timer.ObserveDuration()
					return err
				}
				timer.ObserveDuration()
			case <-t.Dying():
				return nil
			}
		}
	})
}
