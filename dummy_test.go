package sqlt_test

import "github.com/tinywasm/model"

type dummyModel struct{}

func (dummyModel) EncodeFields(w model.FieldWriter) {}
func (dummyModel) DecodeFields(r model.FieldReader) {}
func (dummyModel) IsNil() bool                       { return false }
