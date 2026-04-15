package storage

// nullable helpers for converting Go zero values to SQL NULLs.

func Ns(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func Ni(p *int) interface{} {
	if p == nil {
		return nil
	}
	return *p
}

func Ni64(p *int64) interface{} {
	if p == nil {
		return nil
	}
	return *p
}

func Nf64(p *float64) interface{} {
	if p == nil {
		return nil
	}
	return *p
}
