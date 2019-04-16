package main

import "strconv"

const (
	K = 1024
	M = 1024 * 1024
	G = 1024 * 1024 * 1024
)

type ChunkSize int64

func (c *ChunkSize) MarshalText() (text []byte, err error) { // nolint: unparam
	return []byte(c.String()), nil
}

func (c *ChunkSize) UnmarshalText(text []byte) error {
	return c.Set(string(text))
}

func (c *ChunkSize) Set(value string) error {
	switch value[len(value)-1] {
	case 'K':
		value = value[:len(value)-1]
		i, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return err
		}
		i *= K
		*c = ChunkSize(i)
		return nil
	case 'M':
		value = value[:len(value)-1]
		i, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return err
		}
		i *= M
		*c = ChunkSize(i)
		return nil
	case 'G':
		value = value[:len(value)-1]
		i, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return err
		}
		i *= G
		*c = ChunkSize(i)
		return nil
	default:
		i, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return err
		}
		*c = ChunkSize(i)
		return nil
	}
}

func (c *ChunkSize) String() string {
	i := int64(*c)
	if i == 0 {
		return "0"
	}
	var postfix string
	switch {
	case i%G == 0:
		i /= G
		postfix = "G"
	case i%M == 0:
		i /= M
		postfix = "M"
	case i%K == 0:
		i /= K
		postfix = "K"
	}
	return strconv.FormatInt(i, 10) + postfix
}
