package utils

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
