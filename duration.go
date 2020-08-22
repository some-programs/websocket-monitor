package main

import (
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"time"
)

type Duration time.Duration

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch value := v.(type) {
	case float64:
		*d = Duration(time.Duration(value))
		return nil
	case string:
		tmp, err := time.ParseDuration(value)
		if err != nil {
			return err
		}
		*d = Duration(tmp)
		return nil
	default:
		return errors.New("invalid duration")
	}
}

func (d *Duration) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var v interface{}
	if err := unmarshal(&v); err != nil {
		return err
	}
	switch value := v.(type) {
	case float64:
		*d = Duration(time.Duration(value))
		return nil
	case string:
		tmp, err := time.ParseDuration(value)
		if err != nil {
			return err
		}
		*d = Duration(tmp)
		return nil
	default:
		return errors.New("invalid duration")
	}

}

func (d Duration) MarshalYAML() (interface{}, error) {
	return time.Duration(d).String(), nil
}

func (d Duration) D() time.Duration {
	return time.Duration(d)
}

func (d Duration) MS() DurationMS {
	return DurationMS(d)
}

type DurationMS time.Duration

func (d DurationMS) MarshalJSON() ([]byte, error) {
	return json.Marshal(math.Round(((float64(d) / float64(time.Millisecond)) * 100.0)) / 100.0)
}

func (d *DurationMS) UnmarshalJSON(b []byte) error {
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch value := v.(type) {
	case float64:
		value = value * float64(time.Millisecond)
		*d = DurationMS(time.Duration(value))
		return nil
	case string:
		tmp, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return err
		}
		tmp = tmp * float64(time.Millisecond)
		*d = DurationMS(tmp)
		return nil
	default:
		return errors.New("invalid duration")
	}
}

func (d *DurationMS) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var v interface{}
	if err := unmarshal(&v); err != nil {
		return err
	}
	switch value := v.(type) {
	case float64:
		value = value * float64(time.Millisecond)
		*d = DurationMS(time.Duration(value))
		return nil
	case string:
		tmp, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return err
		}
		tmp = tmp * float64(time.Millisecond)
		*d = DurationMS(tmp)
		return nil
	default:
		return errors.New("invalid duration")
	}

}

func (d DurationMS) MarshalYAML() (interface{}, error) {
	return math.Round(((float64(d) / float64(time.Millisecond)) * 100.0)) / 100.0, nil
}

func (d DurationMS) D() time.Duration {
	return time.Duration(d)
}
