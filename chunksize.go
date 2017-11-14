package main

import "strconv"

type ChunkSize int64

func (c *ChunkSize) UnmarshalText(text []byte) error {
	s := string(text)
	switch s[len(s)-1] {
	case 'K':
		s = s[:len(s)-1]
		i, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return err
		}
		i *= 1024
		*c = ChunkSize(i)
		return nil
	case 'M':
		s = s[:len(s)-1]
		i, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return err
		}
		i *= (1024 * 1024)
		*c = ChunkSize(i)
		return nil
	case 'G':
		s = s[:len(s)-1]
		i, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return err
		}
		i *= (1024 * 1024 * 1024)
		*c = ChunkSize(i)
		return nil
	default:
		i, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return err
		}
		*c = ChunkSize(i)
		return nil
	}
}
