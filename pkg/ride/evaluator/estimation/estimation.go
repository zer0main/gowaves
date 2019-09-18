package estimation

import (
	"github.com/pkg/errors"
	"github.com/wavesplatform/gowaves/pkg/ride/evaluator/ast"
)

type context struct {
	expressions map[string]ast.Expr
	references  map[string]struct{}
}

func newContext(variables map[string]ast.Expr) *context {
	r := make(map[string]struct{}, len(variables))
	e := make(map[string]ast.Expr, len(variables))
	for k, v := range variables {
		r[k] = struct{}{}
		e[k] = v
	}
	e["height"] = ast.NewLong(0)
	r["height"] = struct{}{}
	e["tx"] = ast.NewObject(map[string]ast.Expr{})
	r["tx"] = struct{}{}
	return &context{
		expressions: e,
		references:  r,
	}
}

func (c *context) clone() *context {
	e := make(map[string]ast.Expr, len(c.expressions))
	for k, v := range c.expressions {
		e[k] = v
	}
	r := make(map[string]struct{}, len(c.references))
	for k, v := range c.references {
		r[k] = v
	}
	return &context{
		expressions: e,
		references:  r,
	}
}

type EstimatorV1 struct {
	catalogue *Catalogue
	context   *context
}

func NewEstimatorV1(catalogue *Catalogue, variables map[string]ast.Expr) *EstimatorV1 {
	return &EstimatorV1{
		catalogue: catalogue,
		context:   newContext(variables),
	}
}

func (e *EstimatorV1) Estimate(script ast.Script) (int64, error) {
	verifierCost, err := e.estimate(script.Verifier)
	if err != nil {
		return 0, errors.Wrap(err, "estimation")
	}
	//TODO: add estimation of other entry points and take max among them and the verifier
	return verifierCost, nil
}

func (e *EstimatorV1) estimate(expr ast.Expr) (int64, error) {
	switch expression := expr.(type) {
	case *ast.StringExpr, *ast.LongExpr, *ast.BooleanExpr, *ast.BytesExpr:
		return 1, nil

	case ast.Exprs:
		var total int64 = 0
		for _, item := range expression {
			c, err := e.estimate(item)
			if err != nil {
				return 0, err
			}
			total += c
		}
		return total, nil

	case *ast.Block:
		tmp := e.context.clone()
		e.context.expressions[expression.Let.Name] = expression.Let.Value
		delete(e.context.references, expression.Let.Name)
		bc, err := e.estimate(expression.Body)
		if err != nil {
			return 0, err
		}
		e.context = tmp
		return bc + 5, nil

	case *ast.FuncCallExpr:
		cc, err := e.estimate(expression.Func)
		if err != nil {
			return 0, err
		}
		return cc, nil

	case *ast.NativeFunction:
		fc, ok := e.catalogue.NativeFunctionCost(expression.FunctionID)
		if !ok {
			return 0, errors.Errorf("no native function %d in scope", expression.FunctionID)
		}
		ac, err := e.estimate(expression.Argv)
		if err != nil {
			return 0, err
		}
		return fc + ac, nil

	case *ast.UserFunctionCall:
		fc, ok := e.catalogue.UserFunctionCost(expression.Name)
		if !ok {
			return 0, errors.Errorf("no user function '%s' in scope", expression.Name)
		}
		ac, err := e.estimate(expression.Argv)
		if err != nil {
			return 0, err
		}
		return fc + ac, nil

	case *ast.RefExpr:
		inner, ok := e.context.expressions[expression.Name]
		if !ok {
			return 0, errors.Errorf("no variable '%s' in context", expression.Name)
		}
		_, ok = e.context.references[expression.Name]
		if !ok {
			ic, err := e.estimate(inner)
			if err != nil {
				return 0, err
			}
			e.context.references[expression.Name] = struct{}{}
			return ic + 2, nil
		}
		return 2, nil

	case *ast.IfExpr:
		cc, err := e.estimate(expression.Condition)
		if err != nil {
			return 0, err
		}
		tmp := e.context.clone()
		tc, err := e.estimate(expression.True)
		if err != nil {
			return 0, err
		}
		trueContext := e.context.clone()
		e.context = tmp
		fc, err := e.estimate(expression.False)
		if err != nil {
			return 0, err
		}
		if tc > fc {
			e.context = trueContext
			return tc + cc + 1, nil
		}
		return fc + cc + 1, nil

	case *ast.GetterExpr:
		c, err := e.estimate(expression.Object)
		if err != nil {
			return 0, err
		}
		return c + 2, nil

	default:
		return 0, nil
	}
}

type Catalogue struct {
	native map[int16]int64
	user   map[string]int64
}

func (c *Catalogue) NativeFunctionCost(id int16) (int64, bool) {
	v, ok := c.native[id]
	return v, ok
}

func (c *Catalogue) UserFunctionCost(id string) (int64, bool) {
	v, ok := c.user[id]
	return v, ok
}

func NewCatalogueV2() *Catalogue {
	c := &Catalogue{
		native: make(map[int16]int64),
		user:   make(map[string]int64),
	}

	c.native[0] = 1
	c.native[1] = 1
	c.native[2] = 1
	c.native[100] = 1
	c.native[101] = 1
	c.native[102] = 1
	c.native[103] = 1
	c.native[104] = 1
	c.native[105] = 1
	c.native[106] = 1
	c.native[107] = 1
	c.native[200] = 1
	c.native[201] = 1
	c.native[202] = 1
	c.native[203] = 10
	c.native[300] = 10
	c.native[303] = 1
	c.native[304] = 1
	c.native[305] = 1
	c.native[400] = 2
	c.native[401] = 2
	c.native[410] = 1
	c.native[411] = 1
	c.native[412] = 1
	c.native[420] = 1
	c.native[421] = 1
	c.native[500] = 100
	c.native[501] = 10
	c.native[502] = 10
	c.native[503] = 10
	c.native[600] = 10
	c.native[601] = 10
	c.native[602] = 10
	c.native[603] = 10
	c.native[1000] = 100
	c.native[1001] = 100
	c.native[1003] = 100
	c.native[1040] = 10
	c.native[1041] = 10
	c.native[1042] = 10
	c.native[1043] = 10
	c.native[1050] = 100
	c.native[1051] = 100
	c.native[1052] = 100
	c.native[1053] = 100
	c.native[1060] = 100

	c.user["throw"] = 2
	c.user["addressFromString"] = 124
	c.user["!="] = 26
	c.user["isDefined"] = 35
	c.user["extract"] = 13
	c.user["dropRightBytes"] = 19
	c.user["takeRightBytes"] = 19
	c.user["takeRight"] = 19
	c.user["dropRight"] = 19
	c.user["!"] = 11
	c.user["-"] = 9
	c.user["getInteger"] = 10
	c.user["getBoolean"] = 10
	c.user["getBinary"] = 10
	c.user["getString"] = 10
	c.user["addressFromPublicKey"] = 82
	c.user["wavesBalance"] = 109

	// Type constructors, type constructor cost equals to the number of it's parameters
	c.user["Address"] = 1
	c.user["Alias"] = 1
	c.user["DataEntry"] = 2

	return c
}

func NewCatalogueV3() *Catalogue {
	c := NewCatalogueV2()

	// New native functions
	c.native[108] = 100
	c.native[109] = 100
	c.native[504] = 300
	c.native[604] = 10
	c.native[605] = 10
	c.native[1004] = 100
	c.native[1005] = 100
	c.native[1006] = 100
	c.native[700] = 30
	c.native[1061] = 10
	c.native[1070] = 100
	c.native[1100] = 2
	c.native[1200] = 20
	c.native[1201] = 10
	c.native[1202] = 10
	c.native[1203] = 20
	c.native[1204] = 20
	c.native[1205] = 100
	c.native[1206] = 20
	c.native[1207] = 20
	c.native[1208] = 20

	// Cost updates for existing user functions
	c.user["throw"] = 1
	c.user["isDefined"] = 1
	c.user["!="] = 1
	c.user["!"] = 1
	c.user["-"] = 1

	// Constructors for simple types
	c.user["Ceiling"] = 0
	c.user["Floor"] = 0
	c.user["HalfEven"] = 0
	c.user["Down"] = 0
	c.user["Up"] = 0
	c.user["HalfUp"] = 0
	c.user["HalfDown"] = 0
	c.user["NoAlg"] = 0
	c.user["Md5"] = 0
	c.user["Sha1"] = 0
	c.user["Sha224"] = 0
	c.user["Sha256"] = 0
	c.user["Sha384"] = 0
	c.user["Sha512"] = 0
	c.user["Sha3224"] = 0
	c.user["Sha3256"] = 0
	c.user["Sha3384"] = 0
	c.user["Sha3512"] = 0
	c.user["Unit"] = 0

	// New user functions
	c.user["@extrNative(1040)"] = 10
	c.user["@extrNative(1041)"] = 10
	c.user["@extrNative(1042)"] = 10
	c.user["@extrNative(1043)"] = 10
	c.user["@extrNative(1050)"] = 100
	c.user["@extrNative(1051)"] = 100
	c.user["@extrNative(1052)"] = 100
	c.user["@extrNative(1053)"] = 100
	c.user["@extrUser(getInteger)"] = 10
	c.user["@extrUser(getBoolean)"] = 10
	c.user["@extrUser(getBinary)"] = 10
	c.user["@extrUser(getString)"] = 10
	c.user["@extrUser(addressFromString)"] = 124
	c.user["parseIntValue"] = 20
	c.user["value"] = 13
	c.user["valueOrErrorMessage"] = 13

	return c
}