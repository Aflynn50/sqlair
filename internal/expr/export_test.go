package expr

func PreparedSQL(pe *PreparedExpr) string {
	return pe.sql()
}
