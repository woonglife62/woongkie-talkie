package redisclient

// import (
// 	"fmt"
// 	"log"

// 	"github.com/gomodule/redigo/redis"
// )

// func connection() redis.Conn {
// 	c, err := redis.Dial("tcp", "redis:6379")
// 	if err != nil {
// 		log.Fatal(err.Error())
// 	}

// 	// Connection Check.
// 	_, err = redis.String(c.Do("PING"))
// 	if err != nil {
// 		c.Close()
// 		log.Fatal(err.Error())
// 	}
// 	//fmt.Printf("PING Response = %s\n", pong)

// 	return c
// }

// func redisDo(commandName string, args ...interface{}) ([]string, error) {
// 	conn := connection()
// 	defer conn.Close()
// 	result := new([]string)
// 	reply, err := conn.Do(commandName, args...)
// 	if err != nil {
// 		*result = append(*result, "")
// 		return *result, err
// 	}
// 	switch reply := reply.(type) {
// 	case []byte:
// 		*result = append(*result, string(reply))
// 		return *result, nil
// 	case string:
// 		*result = append(*result, reply)
// 		return *result, nil
// 	case nil:
// 		*result = append(*result, "")
// 		return *result, redis.ErrNil
// 	case redis.Error:
// 		*result = append(*result, "")
// 		return *result, reply
// 	case int64:
// 		*result = append(*result, string(fmt.Sprint(reply)))
// 		return *result, nil
// 	case []interface{}:
// 		for i := range reply {
// 			*result = append(*result, string(reply[i].([]byte))) // type 강제하기
// 		}
// 		return *result, nil
// 	}

// 	*result = append(*result, "")
// 	return *result, fmt.Errorf("redigo: unexpected type for String, got type %T", reply)

// }

// func Insert(key string, value string) (reply []string, err error) {
// 	return redisDo("SET", key, value)
// }

// func Delete(key string) (reply []string, err error) {
// 	return redisDo("DEL", key)
// }

// func Get(key string) (reply []string, err error) {
// 	return redisDo("GET", key)
// }

// func GetAllKeys() (reply []string, err error) {
// 	return redisDo("KEYS", "*")
// }

// func HashSet(key string, field string, value string) (reply []string, err error) {
// 	return redisDo("HSET", key, field, value)
// }

// func ListLpush(key string, element string) (reply []string, err error) {
// 	return redisDo("LPUSH", key, element)
// }

// func ListRpush(key string, element string) (reply []string, err error) {
// 	return redisDo("RPUSH", key, element)
// }

// func ListAllLrange(key string) (reply []string, err error) {
// 	return redisDo("LRANGE", key, 0, -1)
// }

// func ListAllRrange(key string) (reply []string, err error) {
// 	return redisDo("RRANGE", key, 0, -1)
// }
