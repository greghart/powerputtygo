package utilp

import "iter"

func Map[In any, Out any](slice []In, mapper func(v In) Out) []Out {
	results := make([]Out, len(slice))
	for i, v := range slice {
		results[i] = mapper(v)
	}
	return results
}

func MapSeq[In any, Out any](seq iter.Seq[In], mapper func(v In) Out) iter.Seq[Out] {
	return func(yield func(Out) bool) {
		for v := range seq {
			if !yield(mapper(v)) {
				return // stop iteration if yield returns false
			}
		}
	}
}

func MapSeq2[InK any, InV any, Out any](seq iter.Seq2[InK, InV], mapper func(k InK, v InV) Out) iter.Seq[Out] {
	return func(yield func(Out) bool) {
		for k, v := range seq {
			if !yield(mapper(k, v)) {
				return // stop iteration if yield returns false
			}
		}
	}
}
