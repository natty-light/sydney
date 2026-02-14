package evaluator

import (
	"sydney/object"
)

var builtIns = map[string]*object.BuiltIn{
	"len":    object.GetBuiltInByName("len"),
	"print":  object.GetBuiltInByName("print"),
	"first":  object.GetBuiltInByName("first"),
	"last":   object.GetBuiltInByName("last"),
	"rest":   object.GetBuiltInByName("rest"),
	"append": object.GetBuiltInByName("append"),
	"slice":  object.GetBuiltInByName("slice"),
	"keys":   object.GetBuiltInByName("keys"),
	"values": object.GetBuiltInByName("values"),
}
