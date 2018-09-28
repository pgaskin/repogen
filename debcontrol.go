package main

import (
	"fmt"
	"strings"
	"unicode"
)

// Control represents a Debian control file.
type Control struct {
	Values map[string]string
	Order  []string
}

// NewControl returns an empty Debian control file.
func NewControl() *Control {
	return &Control{
		Values: map[string]string{},
		Order:  []string{},
	}
}

// NewControlFromString parses a Debian control file.
func NewControlFromString(in string) (*Control, error) {
	c := Control{
		Values: map[string]string{},
		Order:  []string{},
	}

	var cur string
	lines := strings.Split(strings.Trim(strings.Replace(in, "\r\n", "\n", -1), "\n")+"\n", "\n") // normalize newline format, trailing newlines
	for n, line := range lines {
		switch {
		// comment
		case strings.HasPrefix(line, "#"):
			continue

		// line continuation
		case strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t"):
			if cur == "" {
				return nil, fmt.Errorf("unexpected continuation line in control at line %d", n+1)
			}

			line = strings.TrimRightFunc(line[1:], unicode.IsSpace)
			if line == "." {
				line = "" // a dot is a placeholder for a blank line
			}

			if c.Values[cur] == "" {
				c.Values[cur] += line + "\n"
			} else {
				if !strings.HasSuffix(c.Values[cur], "\n") {
					c.Values[cur] += "\n" // make sure there is a newline to append after
				}
				c.Values[cur] += line + "\n"
			}

		// key-value pair
		case strings.Contains(line, ":"):
			kv := strings.SplitN(line, ":", 2)
			cur = strings.TrimSpace(kv[0])
			val := strings.TrimSpace(kv[1])
			c.Order = append(c.Order, cur)
			c.Values[cur] = val

		// end of block
		case line == "":
			if n+1 != len(lines) {
				return nil, fmt.Errorf("expected end of control block at line %d", n+1)
			}
			break

		// unexpected
		default:
			return nil, fmt.Errorf("expected key-value pair at line %d", n+1)
		}
	}

	return &c, nil
}

// String encodes to the Debian control format.
func (c *Control) String() string {
	if len(c.Values) != len(c.Order) {
		panic("control values length differs from order")
	}

	var b = new(strings.Builder)
	for _, key := range c.Order {
		val := c.Values[key]
		val = strings.Replace(val, "\n", "\n ", -1)       // line continuations
		val = strings.Replace(val, "\n \n", "\n .\n", -1) // blank line placeholder
		val = strings.TrimSuffix(val, "\n ")              // prevent double newline after multi-line values
		fmt.Fprintf(b, "%s: %s\n", key, val)
	}
	return b.String()
}

// Get gets the value of a control variable.
func (c *Control) Get(key string) (string, bool) {
	val, ok := c.Values[key]
	return val, ok
}

// MustGet gets the value of a control variable or panics if it doesn't exist.
func (c *Control) MustGet(key string) string {
	val, ok := c.Values[key]
	if !ok {
		panic("no such key " + key)
	}
	return val
}

// MightGet gets the value of a control variable or returns an empty string.
func (c *Control) MightGet(key string) string {
	val, _ := c.Values[key]
	return val
}

// Set sets the value of a control variable.
func (c *Control) Set(key, value string) {
	c.Values[key] = value
	for _, okey := range c.Order {
		if okey == key {
			return
		}
	}
	c.Order = append(c.Order, key)
	return
}

// MoveToOrderStart moves a key to the start of the control block.
func (c *Control) MoveToOrderStart(key string) bool {
	for i, okey := range c.Order {
		if okey == key {
			c.Order = append([]string{key}, append(c.Order[:i], c.Order[i+1:]...)...)
			return true
		}
	}
	return false
}

// Clone clones the control block.
func (c *Control) Clone() *Control {
	nc := &Control{
		Values: map[string]string{},
		Order:  make([]string, len(c.Order)),
	}
	for key, val := range c.Values {
		nc.Values[key] = val
	}
	copy(nc.Order, c.Order)
	return nc
}
