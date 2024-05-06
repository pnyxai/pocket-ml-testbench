package utils

import (
	"math/rand"
	"time"
)

// KeyValToMap Converts key-value pairs to map.
func KeyValToMap(kvPairs ...interface{}) map[string]interface{} {
	kvMap := make(map[string]interface{})
	for i := 0; i+1 < len(kvPairs); i += 2 {
		key, keyOk := kvPairs[i].(string)
		value := kvPairs[i+1]
		if keyOk {
			kvMap[key] = value
		}
	}
	return kvMap
}

// StringInSlice checks if a string is present in a slice of strings.
// Returns true if the string is found, otherwise false.
func StringInSlice(str string, list []string) bool {
	for _, v := range list {
		if v == str {
			return true
		}
	}
	return false
}

// MaxInt returns the maximum of two integers.
func MaxInt(x, y int) int {
	if x > y {
		return x
	}
	return y
}

// MinInt returns the minimum of two integers.
func MinInt(x, y int) int {
	if x < y {
		return x
	}
	return y
}

func GetMaxInt64FromArray(numbers []int64) (max int64) {
	for _, number := range numbers {
		if number > max {
			max = number
		}
	}

	return max
}

func GetRandomFromSlice[T interface{}](slice []T) *T {
	s := rand.NewSource(time.Now().UnixNano())
	r := rand.New(s)
	return &slice[r.Intn(len(slice))]
}

func InterfaceSlice[T interface{}](elements []T) []interface{} {
	out := make([]interface{}, len(elements))
	for i, v := range elements {
		out[i] = v
	}
	return out
}
