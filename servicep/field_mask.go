package servicep

// FieldMask is the powerputtyp version of a protobuf style [FieldMask].
// Works well with both `fieldmaskpb.FieldMask`, as well as being useful for HTTP APIs.
// Supports nil receiver when makes sense, so you can easily have an optional field mask.
// TODO: Support `foo.*`, `foo.(id,name,field)`
type FieldMask struct {
	byPath map[string]any
}

func NewFieldMask(paths []string) *FieldMask {
	byPath := make(map[string]any, len(paths))
	for _, path := range paths {
		byPath[path] = nil
	}
	return &FieldMask{
		byPath: byPath,
	}
}

// pather interface is implemented by fieldmaskpb.FieldMask
type pather interface {
	GetPaths() []string
}

func NewFieldMaskFromPB(p pather) *FieldMask {
	if p == nil {
		return NewFieldMask([]string{})
	}
	return NewFieldMask(p.GetPaths())
}

func (m *FieldMask) Add(field string) {
	m.byPath[field] = nil
}

// Has returns whether given field is in the mask.
func (m *FieldMask) Has(field string) bool {
	if m == nil {
		return true
	}
	_, ok := m.byPath[field]
	return ok
}

// HasAny returns whether any of the given fields are in the mask.
func (m *FieldMask) HasAny(fields ...string) bool {
	for _, f := range fields {
		if m.Has(f) {
			return true
		}
	}
	return false
}

// MaskGet returns a value IFF it's in the field mask
func MaskGet[T any](mask *FieldMask, field string, value T) (T, bool) {
	var z T
	if !mask.Has(field) {
		return z, false
	}
	return z, true
}

// ZeroToPtr maps values to pointers to that value, returning nil if the value is the zero value
func ZeroToPtr[T comparable](v T) *T {
	var zero T
	if zero == v {
		return nil
	}
	return &v
}

// PtrToZero maps pointers to values to the value, or the zero value for a nil pointer.
func PtrToZero[T comparable](v *T) T {
	if v == nil {
		var zero T
		return zero
	}
	return *v
}
