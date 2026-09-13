package tenant

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-redis/redis/v8"
)

// RedisScope fails closed for data commands without a workspace. A single
// client and connection pool can safely serve requests from every workspace.
type RedisScope struct{}

func (RedisScope) BeforeProcess(ctx context.Context, cmd redis.Cmder) (context.Context, error) {
	return ctx, scopeRedisCommand(ctx, cmd)
}

func (RedisScope) AfterProcess(ctx context.Context, cmd redis.Cmder) error {
	if scan, ok := cmd.(*redis.ScanCmd); ok && scan.Err() == nil {
		prefix, err := Key(ctx, "")
		if err != nil {
			return err
		}
		keys, cursor := scan.Val()
		for i := range keys {
			if !strings.HasPrefix(keys[i], prefix) {
				return ErrMismatch
			}
			keys[i] = strings.TrimPrefix(keys[i], prefix)
		}
		scan.SetVal(keys, cursor)
	}
	return nil
}

func (hook RedisScope) BeforeProcessPipeline(ctx context.Context, commands []redis.Cmder) (context.Context, error) {
	for _, cmd := range commands {
		if err := scopeRedisCommand(ctx, cmd); err != nil {
			return ctx, err
		}
	}
	return ctx, nil
}

func (hook RedisScope) AfterProcessPipeline(ctx context.Context, commands []redis.Cmder) error {
	for _, cmd := range commands {
		if err := hook.AfterProcess(ctx, cmd); err != nil {
			return err
		}
	}
	return nil
}

func scopeRedisCommand(ctx context.Context, cmd redis.Cmder) error {
	name := strings.ToLower(cmd.Name())
	switch name {
	case "ping", "hello", "auth", "select", "client", "command", "multi", "exec", "discard":
		return nil
	}
	prefix, err := Key(ctx, "")
	if err != nil {
		return err
	}
	args := cmd.Args()
	var positions []int
	switch name {
	case "get", "getex", "set", "setnx", "getset", "expire", "pexpire", "ttl", "pttl", "persist", "incr", "incrby", "decr", "decrby", "hget", "hmget", "hgetall", "hset", "hmset", "hdel", "hexists", "hincrby", "llen", "lrange", "lpush", "rpush", "ltrim", "sadd", "smembers", "zadd", "zremrangebyscore", "zcard", "zrange":
		positions = []int{1}
	case "del", "unlink", "mget", "exists":
		for i := 1; i < len(args); i++ {
			positions = append(positions, i)
		}
	case "eval", "evalsha":
		if len(args) < 3 {
			return fmt.Errorf("invalid Redis script command")
		}
		count, err := strconv.Atoi(fmt.Sprint(args[2]))
		if err != nil || count < 1 || count > len(args)-3 {
			return fmt.Errorf("Redis scripts require explicit tenant keys")
		}
		for i := range count {
			positions = append(positions, i+3)
		}
	case "scan":
		for i := 2; i+1 < len(args); i++ {
			if strings.EqualFold(fmt.Sprint(args[i]), "match") {
				positions = []int{i + 1}
				break
			}
		}
		if len(positions) == 0 {
			return fmt.Errorf("Redis scans require a tenant key pattern")
		}
	default:
		return fmt.Errorf("Redis command %q is not permitted in a workspace", name)
	}
	for _, position := range positions {
		if position >= len(args) {
			return fmt.Errorf("invalid Redis command arguments")
		}
		key := fmt.Sprint(args[position])
		if strings.HasPrefix(key, prefix) {
			continue
		}
		if strings.HasPrefix(key, "tenant:") {
			return ErrMismatch
		}
		args[position] = prefix + key
	}
	return nil
}
