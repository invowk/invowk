// SPDX-License-Identifier: MPL-2.0

package validatedelegation

import "fmt"

// --- Types with Validate() for use as fields ---

type Name string

func (n Name) Validate() error {
	if n == "" {
		return fmt.Errorf("empty name")
	}
	return nil
}

func (n Name) String() string { return string(n) }

type Mode string

func (m Mode) Validate() error {
	if m == "" {
		return fmt.Errorf("empty mode")
	}
	return nil
}

func (m Mode) String() string { return string(m) }

type Level string

func (l Level) Validate() error {
	if l == "" {
		return fmt.Errorf("empty level")
	}
	return nil
}

func (l Level) String() string { return string(l) }

// --- Complete delegation (all 3 fields) — should NOT be flagged ---

//goplint:validate-all
type CompleteConfig struct {
	FieldName  Name
	FieldMode  Mode
	FieldLevel Level
}

func (c *CompleteConfig) Validate() error {
	if err := c.FieldName.Validate(); err != nil {
		return err
	}
	if err := c.FieldMode.Validate(); err != nil {
		return err
	}
	if err := c.FieldLevel.Validate(); err != nil {
		return err
	}
	return nil
}

// --- Incomplete delegation (misses FieldLevel) — should be flagged ---

//goplint:validate-all
type IncompleteConfig struct { // want `validatedelegation\.IncompleteConfig\.Validate\(\) does not delegate to field FieldLevel which has Validate\(\)`
	FieldName  Name
	FieldMode  Mode
	FieldLevel Level
}

func (c *IncompleteConfig) Validate() error {
	if err := c.FieldName.Validate(); err != nil {
		return err
	}
	if err := c.FieldMode.Validate(); err != nil {
		return err
	}
	// FieldLevel.Validate() is missing!
	return nil
}

// --- No validate-all directive — should NOT be flagged even with missing delegation ---

type NoDirectiveConfig struct {
	FieldName Name
	FieldMode Mode
}

func (c *NoDirectiveConfig) Validate() error {
	// Only validates FieldName, skips FieldMode — but no directive, so not checked.
	if err := c.FieldName.Validate(); err != nil {
		return err
	}
	return nil
}

// --- Delegation via intermediate variable — should NOT be flagged ---

//goplint:validate-all
type AliasConfig struct {
	FieldName Name
	FieldMode Mode
}

func (c *AliasConfig) Validate() error {
	name := c.FieldName
	if err := name.Validate(); err != nil {
		return err
	}
	mode := c.FieldMode
	if err := mode.Validate(); err != nil {
		return err
	}
	return nil
}

// --- Value receiver delegation — should NOT be flagged ---

//goplint:validate-all
type ValueReceiverConfig struct {
	FieldName Name
	FieldMode Mode
}

func (c ValueReceiverConfig) Validate() error {
	if err := c.FieldName.Validate(); err != nil {
		return err
	}
	if err := c.FieldMode.Validate(); err != nil {
		return err
	}
	return nil
}

// --- Embedded field delegation — complete ---

//goplint:validate-all
type EmbeddedComplete struct {
	Name
	FieldMode Mode
}

func (c *EmbeddedComplete) Validate() error {
	if err := c.Name.Validate(); err != nil {
		return err
	}
	if err := c.FieldMode.Validate(); err != nil {
		return err
	}
	return nil
}

// --- Embedded field delegation — incomplete ---

//goplint:validate-all
type EmbeddedIncomplete struct { // want `validatedelegation\.EmbeddedIncomplete\.Validate\(\) does not delegate to field Name which has Validate\(\)`
	Name
	FieldMode Mode
}

func (c *EmbeddedIncomplete) Validate() error {
	// Name.Validate() is missing!
	if err := c.FieldMode.Validate(); err != nil {
		return err
	}
	return nil
}

// --- Mix of validatable and non-validatable fields ---

//goplint:validate-all
type MixedFieldConfig struct { // want `validatedelegation\.MixedFieldConfig\.Validate\(\) does not delegate to field FieldMode which has Validate\(\)`
	FieldName Name
	FieldMode Mode
	plain     int // want `struct field validatedelegation\.MixedFieldConfig\.plain uses primitive type int`
}

func (c *MixedFieldConfig) Validate() error {
	if err := c.FieldName.Validate(); err != nil {
		return err
	}
	// FieldMode.Validate() is missing!
	_ = c.plain
	return nil
}
