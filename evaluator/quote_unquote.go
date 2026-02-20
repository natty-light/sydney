package evaluator

import (
	"fmt"
	"sydney/ast"
	"sydney/object"
	"sydney/token"
)

func quote(node ast.Node, s *object.Scope) object.Object {
	node = evalUnquoteCalls(node, s)
	return &object.Quote{Node: node}
}

func evalUnquoteCalls(quoted ast.Node, s *object.Scope) ast.Node {
	return ast.Modify(quoted, func(node ast.Node) ast.Node {
		if !isUnquoteCall(node) {
			return node
		}

		call, ok := node.(*ast.CallExpr)
		if !ok {
			return node
		}

		if len(call.Arguments) != 1 {
			return node
		}

		unquoted := Eval(call.Arguments[0], s)
		return convertObjectToASTNode(unquoted)
	})
}

func isUnquoteCall(node ast.Node) bool {
	callExpr, ok := node.(*ast.CallExpr)
	if !ok {
		return false
	}
	return callExpr.Function.TokenLiteral() == "unquote"
}

func convertObjectToASTNode(obj object.Object) ast.Node {
	switch obj := obj.(type) {
	case *object.Integer:
		t := token.Token{
			Type:    token.Integer,
			Literal: fmt.Sprintf("%d", obj.Value),
		}
		return &ast.IntegerLiteral{Token: t, Value: obj.Value}
	case *object.Float:
		t := token.Token{
			Type:    token.Float,
			Literal: fmt.Sprintf("%f", obj.Value),
		}
		return &ast.FloatLiteral{Token: t, Value: obj.Value}
	case *object.Boolean:
		var t token.Token
		if obj.Value {
			t = token.Token{Type: token.True, Literal: "true"}
		} else {
			t = token.Token{Type: token.False, Literal: "false"}
		}
		return &ast.BooleanLiteral{Token: t, Value: obj.Value}
	case *object.Quote:
		return obj.Node
	default:
		return nil
	}

}
