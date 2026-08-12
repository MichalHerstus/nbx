package core

import (
	validation "github.com/pocketbase/ozzo-validation/v4"
)

var _ optionsValidator = (*collectionBaseOptions)(nil)

// collectionBaseOptions defines the options for the "base" type collection.
type collectionBaseOptions struct {
	// DataSource defines the source of the collection data (local by default).
	DataSource DataSource `form:"datasource" json:"datasource"`
}

func (o *collectionBaseOptions) validate(cv *collectionValidator) error {
	return validation.ValidateStruct(o,
		validation.Field(&o.DataSource),
	)
}

// GetDataSource returns the collection datasource configuration.
//
// If the collection is not a "base" type or the datasource block is empty,
// it returns the default local datasource.
func (m *Collection) GetDataSource() DataSource {
	if !m.IsBase() {
		return NewDefaultDataSource()
	}

	if m.DataSource.Type == "" {
		return NewDefaultDataSource()
	}

	return m.DataSource
}
