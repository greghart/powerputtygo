package mapperp

// Map rows, while potentially using context for side effects
type Mapper[Row any, Out any] func(out *Out, row *Row, i int)         // A row mapper maps rows onto an output entity
type Identifier[E any, ID comparable] func(e *E) ID                   // identify entities by their ID
type DataGetter[In any, Out any] func(row *In) *Out                   // get data from a row
type MapperDeferred[Row any, Out any] func(out *Out, row *Row, i int) // A row mapper maps rows onto an output entity

// One maps multiple rows to a single output.
func One[Row any, Out any](getData DataGetter[Row, Out], rest ...Mapper[Row, Out]) Mapper[Row, Out] {
	once := false
	return All(
		append(
			[]Mapper[Row, Out]{func(out *Out, row *Row, i int) {
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

func Slice[Row any, Out any](
	getID Identifier[Out, int64],
	getData DataGetter[Row, Out],
	rest ...Mapper[Row, []Out],
) Mapper[Row, []Out] {
	currID := int64(0)
	return All(
		append(
			[]Mapper[Row, []Out]{func(out *[]Out, row *Row, i int) {
				// datum check
				datum := getData(row)
				if datum == nil {
					return
				}
				// check new entity based on ID
				id := getID(datum)
				if id == currID {
					return
				}
				// initialize if nil
				if *out == nil {
					*out = []Out{}
				}
				*out = append(*out, *datum)
				currID = id
			}},
			rest...,
		)...,
	)
}

// Inner sets up a sub mapper into our current output.
func Inner[Row any, Out any, In any](
	getInner func(e *Out) *In,
	inner ...Mapper[Row, In],
) Mapper[Row, Out] {
	return func(out *Out, row *Row, i int) {
		if out == nil {
			return
		}
		sub := getInner(out)
		if sub == nil {
			return
		}
		All(
			inner...,
		)(sub, row, i)
	}
}

func InnerSlice[Row any, Out any, In any](
	getInner func(e *Out) *[]In,
	getID Identifier[In, int64],
	getData DataGetter[Row, In],
	inner ...Mapper[Row, []In],
) Mapper[Row, Out] {
	return Inner(getInner, Slice(getID, getData, inner...))
}

func Last[Row any, Out any](
	inner ...Mapper[Row, Out],
) Mapper[Row, []Out] {
	return func(out *[]Out, row *Row, i int) {
		if out == nil || len(*out) == 0 {
			return
		}
		last := &(*out)[len(*out)-1] // get the last element
		All(
			inner...,
		)(last, row, i)
	}
}

// All just runs all mappers in sequence.
func All[Row any, Out any](
	mappers ...Mapper[Row, Out],
) Mapper[Row, Out] {
	return func(out *Out, row *Row, i int) {
		for _, mapper := range mappers {
			mapper(out, row, i)
		}
	}
}
