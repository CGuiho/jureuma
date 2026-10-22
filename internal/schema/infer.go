package schema

import "github.com/CGuiho/zeze/internal/doc"

// Infer builds a starter schema from structure and JSON types, never values:
// objects get "required" for every present key and additionalProperties
// false; homogeneous arrays become "items"; mixed arrays become
// "prefixItems"; empty arrays become "items": {}. If typesKnown is false
// (encrypted source), leaves get {} (any).
func Infer(v *doc.Value, typesKnown bool) *doc.Value {
	s := infer(v, typesKnown)
	out := doc.NewObject()
	out.Put("$schema", doc.NewString(Draft202012))
	for _, k := range s.Keys {
		out.Put(k, s.M[k])
	}
	return out
}

func infer(v *doc.Value, typesKnown bool) *doc.Value {
	o := doc.NewObject()
	switch v.Kind {
	case doc.Object:
		o.Put("type", doc.NewString("object"))
		props := doc.NewObject()
		req := doc.NewArray()
		for _, k := range v.Keys {
			props.Put(k, infer(v.M[k], typesKnown))
			req.A = append(req.A, doc.NewString(k))
		}
		o.Put("properties", props)
		if len(req.A) > 0 {
			o.Put("required", req)
		}
		o.Put("additionalProperties", doc.NewBool(false))
	case doc.Array:
		o.Put("type", doc.NewString("array"))
		if len(v.A) == 0 {
			o.Put("items", doc.NewObject())
			break
		}
		elems := make([]*doc.Value, len(v.A))
		same := true
		for i, e := range v.A {
			elems[i] = infer(e, typesKnown)
			if i > 0 && !doc.Equal(elems[i], elems[0]) {
				same = false
			}
		}
		if same {
			o.Put("items", elems[0])
		} else {
			o.Put("prefixItems", doc.NewArray(elems...))
		}
	default:
		if typesKnown {
			o.Put("type", doc.NewString(v.Kind.String()))
		}
	}
	return o
}
