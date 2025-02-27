package models

// Module is a struct that represents a module. Used for storing module information.
type Module struct {
	Name        string `bson:"name"`
	Description string `bson:"description"`
}
