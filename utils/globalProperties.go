package utils

import (
	"fmt"
	"github.com/magiconair/properties"
)

var props *properties.Properties

func SetProps(p *properties.Properties) error {
	if props != nil {
		return fmt.Errorf("properties already set")
	}
	props = p
	return nil
}

func GetProps() (*properties.Properties, error) {
	if props == nil {
		return nil, fmt.Errorf("properties not set")
	}
	return props, nil
}
