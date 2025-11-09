package main

import (
	"fmt"
	"strings"

	pgquery "github.com/pganalyze/pg_query_go/v6"
)

// extractNodeConstraint extracts constraint information from any Node type
func extractNodeConstraint(node *pgquery.Node) string {
	if node == nil {
		return ""
	}

	switch expr := node.Node.(type) {
	case *pgquery.Node_AExpr:
		return extractAExprConstraint(expr.AExpr)
	case *pgquery.Node_BoolExpr:
		return extractBoolExprConstraint(expr.BoolExpr)
	case *pgquery.Node_FuncCall:
		return extractFuncCallConstraint(expr.FuncCall)
	case *pgquery.Node_ColumnRef:
		return extractColumnRef(expr.ColumnRef)
	case *pgquery.Node_AConst:
		return extractConstantValue(expr.AConst)
	default:
		return ""
	}
}

func extractAExprConstraint(aexpr *pgquery.A_Expr) string {
	if aexpr == nil {
		return ""
	}

	switch aexpr.GetKind() {
	case pgquery.A_Expr_Kind_AEXPR_OP:
		return extractOpExpr(aexpr)
	case pgquery.A_Expr_Kind_AEXPR_IN:
		return extractInExpr(aexpr)
	case pgquery.A_Expr_Kind_AEXPR_LIKE, pgquery.A_Expr_Kind_AEXPR_ILIKE:
		return extractLikeExpr(aexpr)
	case pgquery.A_Expr_Kind_AEXPR_BETWEEN:
		return extractBetweenExpr(aexpr)
	default:
		return ""
	}
}

func extractOpExpr(aexpr *pgquery.A_Expr) string {
	op := extractOperatorName(aexpr.GetName())
	if op == "" {
		return ""
	}

	leftExpr := extractNodeConstraint(aexpr.GetLexpr())
	rightExpr := extractNodeConstraint(aexpr.GetRexpr())

	// Handle special case where left side might be implicit (like for domain constraints)
	if leftExpr == "" && rightExpr != "" {
		return fmt.Sprintf("%s %s", op, rightExpr)
	}
	if leftExpr != "" && rightExpr != "" {
		return fmt.Sprintf("%s %s %s", leftExpr, op, rightExpr)
	}
	return ""
}

func extractInExpr(aexpr *pgquery.A_Expr) string {
	leftExpr := extractNodeConstraint(aexpr.GetLexpr())

	if rlist := aexpr.GetRexpr().GetList(); rlist != nil {
		var vals []string
		for _, item := range rlist.GetItems() {
			val := extractNodeConstraint(item)
			if val != "" {
				vals = append(vals, val)
			}
		}
		if len(vals) > 0 {
			if leftExpr != "" {
				return fmt.Sprintf("%s IN (%s)", leftExpr, strings.Join(vals, ", "))
			}
			return fmt.Sprintf("IN (%s)", strings.Join(vals, ", "))
		}
	}
	return ""
}

func extractLikeExpr(aexpr *pgquery.A_Expr) string {
	op := extractOperatorName(aexpr.GetName())
	if op == "" {
		return ""
	}

	leftExpr := extractNodeConstraint(aexpr.GetLexpr())
	rightExpr := extractNodeConstraint(aexpr.GetRexpr())

	if rightExpr != "" {
		if leftExpr != "" {
			return fmt.Sprintf("%s %s %s", leftExpr, op, rightExpr)
		}
		return fmt.Sprintf("%s %s", op, rightExpr)
	}
	return ""
}

func extractBetweenExpr(aexpr *pgquery.A_Expr) string {
	leftExpr := extractNodeConstraint(aexpr.GetLexpr())

	if rlist := aexpr.GetRexpr().GetList(); rlist != nil && len(rlist.GetItems()) == 2 {
		lowerVal := extractNodeConstraint(rlist.GetItems()[0])
		upperVal := extractNodeConstraint(rlist.GetItems()[1])

		if lowerVal != "" && upperVal != "" {
			if leftExpr != "" {
				return fmt.Sprintf("%s BETWEEN %s AND %s", leftExpr, lowerVal, upperVal)
			}
			return fmt.Sprintf("BETWEEN %s AND %s", lowerVal, upperVal)
		}
	}
	return ""
}

func extractBoolExprConstraint(boolExpr *pgquery.BoolExpr) string {
	if boolExpr == nil {
		return ""
	}

	var constraints []string
	for _, arg := range boolExpr.GetArgs() {
		constraint := extractNodeConstraint(arg)
		if constraint != "" {
			constraints = append(constraints, constraint)
		}
	}

	if len(constraints) == 0 {
		return ""
	}

	var operator string
	switch boolExpr.GetBoolop() {
	case pgquery.BoolExprType_AND_EXPR:
		operator = " AND "
	case pgquery.BoolExprType_OR_EXPR:
		operator = " OR "
	case pgquery.BoolExprType_NOT_EXPR:
		if len(constraints) == 1 {
			return fmt.Sprintf("NOT (%s)", constraints[0])
		}
		operator = " NOT "
	default:
		operator = " "
	}

	// Wrap complex expressions in parentheses
	if len(constraints) > 1 {
		for i, constraint := range constraints {
			if strings.Contains(constraint, " AND ") || strings.Contains(constraint, " OR ") {
				constraints[i] = fmt.Sprintf("(%s)", constraint)
			}
		}
	}

	return strings.Join(constraints, operator)
}

func extractFuncCallConstraint(funcCall *pgquery.FuncCall) string {
	if funcCall == nil {
		return ""
	}

	// Extract function name
	var funcName string
	if len(funcCall.GetFuncname()) > 0 {
		if nameStr := funcCall.GetFuncname()[0].GetString_(); nameStr != nil {
			funcName = nameStr.GetSval()
		}
	}

	if funcName == "" {
		return ""
	}

	// Extract arguments
	var args []string
	for _, arg := range funcCall.GetArgs() {
		argStr := extractNodeConstraint(arg)
		if argStr != "" {
			args = append(args, argStr)
		}
	}

	if len(args) > 0 {
		return fmt.Sprintf("%s(%s)", funcName, strings.Join(args, ", "))
	}
	return fmt.Sprintf("%s()", funcName)
}

func extractColumnRef(colRef *pgquery.ColumnRef) string {
	if colRef == nil {
		return ""
	}

	var parts []string
	for _, field := range colRef.GetFields() {
		if str := field.GetString_(); str != nil {
			parts = append(parts, str.GetSval())
		}
	}
	return strings.Join(parts, ".")
}

func extractOperatorName(names []*pgquery.Node) string {
	if len(names) > 0 {
		if nameStr := names[0].GetString_(); nameStr != nil {
			return nameStr.GetSval()
		}
	}
	return ""
}

func extractConstantValue(aconst *pgquery.A_Const) string {
	if aconst == nil {
		return ""
	}

	if ival := aconst.GetIval(); ival != nil {
		return fmt.Sprintf("%d", ival.GetIval())
	}
	if fval := aconst.GetFval(); fval != nil {
		return fval.GetFval()
	}
	if sval := aconst.GetSval(); sval != nil {
		return fmt.Sprintf("'%s'", sval.GetSval())
	}
	if boolval := aconst.GetBoolval(); boolval != nil {
		if boolval.GetBoolval() {
			return "true"
		}
		return "false"
	}
	return ""
}
