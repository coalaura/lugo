package lsp

import (
	"testing"
)

func TestParseLuaDoc_Basic(t *testing.T) {
	comments := []byte(`
This is a description.
It spans multiple lines.

@param name string The name of the user
@param age? number Optional age
@return boolean Success status
@deprecated Use new_function instead
	`)

	doc := parseLuaDoc(comments, false)

	expectedDesc := "This is a description.\nIt spans multiple lines."
	if doc.Description != expectedDesc {
		t.Errorf("Expected description %q, got %q", expectedDesc, doc.Description)
	}

	if len(doc.Params) != 2 {
		t.Fatalf("Expected 2 params, got %d", len(doc.Params))
	}

	if doc.Params[0].Name != "name" || doc.Params[0].Type != "string" || doc.Params[0].Desc != "The name of the user" {
		t.Errorf("Param 0 parsed incorrectly: %+v", doc.Params[0])
	}

	// Test optional parameter handling (?)
	if doc.Params[1].Name != "age" || doc.Params[1].Type != "number?" {
		t.Errorf("Param 1 (optional) parsed incorrectly: %+v", doc.Params[1])
	}

	if len(doc.Returns) != 1 {
		t.Fatalf("Expected 1 return, got %d", len(doc.Returns))
	}

	if doc.Returns[0].Type != "boolean" || doc.Returns[0].Desc != "Success status" {
		t.Errorf("Return parsed incorrectly: %+v", doc.Returns[0])
	}

	if !doc.IsDeprecated || doc.DeprecatedMsg != "Use new_function instead" {
		t.Errorf("Deprecated tag parsed incorrectly: %v | %q", doc.IsDeprecated, doc.DeprecatedMsg)
	}
}

func TestParseLuaDoc_ComplexTypesAndFields(t *testing.T) {
	comments := []byte(`
@field public id number
@field private callback fun(a: string, b: number): boolean The callback func
@class MyClass : ParentClass Description of class
	`)

	doc := parseLuaDoc(comments, false)

	if len(doc.Fields) != 2 {
		t.Fatalf("Expected 2 fields, got %d", len(doc.Fields))
	}

	// Should strip "public "
	if doc.Fields[0].Name != "id" || doc.Fields[0].Type != "number" {
		t.Errorf("Field 0 parsed incorrectly: %+v", doc.Fields[0])
	}

	// Should handle spaces inside types
	if doc.Fields[1].Name != "callback" || doc.Fields[1].Type != "fun(a: string, b: number): boolean" || doc.Fields[1].Desc != "The callback func" {
		t.Errorf("Field 1 parsed incorrectly: %+v", doc.Fields[1])
	}

	if doc.Class == nil {
		t.Fatalf("Expected class to be parsed")
	}

	if doc.Class.Name != "MyClass" || doc.Class.Parent != "ParentClass" || doc.Class.Desc != "Description of class" {
		t.Errorf("Class parsed incorrectly: %+v", doc.Class)
	}
}

func TestParseLuaDoc_AdvancedTags(t *testing.T) {
	comments := []byte(`
@alias ID string | number
@generic T : Item
@see other_function
@type number
	`)

	doc := parseLuaDoc(comments, false)

	if doc.Alias == nil || doc.Alias.Name != "ID" || doc.Alias.Type != "string | number" {
		t.Errorf("Alias parsed incorrectly: %+v", doc.Alias)
	}

	if len(doc.Generics) != 1 || doc.Generics[0].Name != "T" || doc.Generics[0].Parent != "Item" {
		t.Errorf("Generic parsed incorrectly: %+v", doc.Generics)
	}

	if len(doc.See) != 1 || doc.See[0] != "other_function" {
		t.Errorf("See parsed incorrectly: %v", doc.See)
	}

	if doc.Type == nil || doc.Type.Type != "number" {
		t.Errorf("Type parsed incorrectly: %+v", doc.Type)
	}
}
