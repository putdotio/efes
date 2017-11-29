package main

import "time"

type Duration time.Duration

func (d *Duration) UnmarshalText(text []byte) error {
	d2, err := time.ParseDuration(string(text))
	*d = Duration(d2)
	return err
}
