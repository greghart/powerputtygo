package mapperp

// Mapper maps rows onto an Out entity.
// They are fully composable to map a deeply nested structure easily.
type Mapper[Row any, Out any] func(out *Out, row *Row)         // A row mapper maps rows onto an output entity
type Identifier[E any, ID comparable] func(e *E) ID            // identify entities by their ID
type DataGetter[In any, Out any] func(row *In) *Out            // get data from a row
type MapperDeferred[Row any, Out any] func(out *Out, row *Row) // A row mapper maps rows onto an output entity

// One maps multiple rows to a single output.
func One[Row any, Out any](getData DataGetter[Row, Out], rest ...Mapper[Row, Out]) Mapper[Row, Out] {
	once := false
	return All(
		append(
			[]Mapper[Row, Out]{func(out *Out, row *Row) {
				if once {
					return
				}
				datum := getData(row)
				if datum == nil {
					return
				}
				*out = *datum
				once = true
			}},
			rest...,
		)...,
	)
}

// Slice maps rows to a slice of outputs, adding a new output whenever a new ID is encountered.
func Slice[Row any, Out any](
	getID Identifier[Out, int64],
	getData DataGetter[Row, Out],
	rest ...Mapper[Row, []Out],
) Mapper[Row, []Out] {
	byID := make(map[int64]bool)
	return All(
		append(
			[]Mapper[Row, []Out]{func(out *[]Out, row *Row) {
				// datum check
				datum := getData(row)
				if datum == nil {
					return
				}
				// check new entity based on ID not being seen or 0 value
				id := getID(datum)
				if id == 0 {
					return
				}
				if _, ok := byID[id]; ok {
					return
				}
				byID[id] = true

				// initialize if nil
				if *out == nil {
					*out = []Out{}
				}
				*out = append(*out, *datum)
			}},
			rest...,
		)...,
	)
}

// Inner sets up a sub mapper into our current output.
func Inner[Row any, Out any, Inner any](
	getInner func(e *Out) *Inner,
	inner ...Mapper[Row, Inner],
) Mapper[Row, Out] {
	return func(out *Out, row *Row) {
		if out == nil {
			return
		}
		sub := getInner(out)
		if sub == nil {
			return
		}
		All(
			inner...,
		)(sub, row)
	}
}

// InnerSlice sets up a sub mapper into our current output, where the target is itself a slice.
func InnerSlice[Row any, Out any, In any](
	getInner func(e *Out) *[]In,
	getID Identifier[In, int64],
	getData DataGetter[Row, In],
	inner ...Mapper[Row, []In],
) Mapper[Row, Out] {
	return Inner(getInner, Slice(getID, getData, inner...))
}

// Take sets up a sub mapper for the current element of a slice output.
func Take[Row any, Out any](
	inner ...Mapper[Row, Out],
) Mapper[Row, []Out] {
	return func(out *[]Out, row *Row) {
		if out == nil || len(*out) == 0 {
			return
		}
		last := &(*out)[len(*out)-1] // get the last element
		All(
			inner...,
		)(last, row)
	}
}

// All just runs all mappers in sequence.
func All[Row any, Out any](
	mappers ...Mapper[Row, Out],
) Mapper[Row, Out] {
	return func(out *Out, row *Row) {
		for _, mapper := range mappers {
			mapper(out, row)
		}
	}
}
