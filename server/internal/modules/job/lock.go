package job

import (
	"context"
	"crypto/rand"
	"time"

	"pet-manager/server/internal/pkg/errcode"
)

// acquireRunLocks 抢占本次执行所需的 Redis 锁，返回释放函数。
//
// 设计要点：
//   - 无论运行期间如何修改执行配置，都先锁定不可修改的任务编码；
//     系统方法额外共享方法锁，兼容原有预置任务锁键，避免同一方法在多个任务间重入；
//   - 每把锁的值是本次随机 token，释放时用 Lua CAS 比对后再删，
//     防止误删已过期后被他人重建的同名锁；
//   - 任一把锁抢占失败即释放已获得的锁并返回冲突错误（任务或方法正在执行）。
func (s *Service) acquireRunLocks(ctx context.Context, code, identity string) (func(), error) {
	identities := []string{"job:" + code}
	if identity != identities[0] {
		identities = append(identities, identity)
	}
	token := rand.Text()
	keys := make([]string, 0, len(identities))
	release := func() {
		releaseCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		for _, key := range keys {
			_ = s.cache.Raw().Eval(releaseCtx, `if redis.call('get', KEYS[1]) == ARGV[1] then return redis.call('del', KEYS[1]) else return 0 end`, []string{key}, token).Err()
		}
	}
	for _, value := range identities {
		key := s.cache.Key("lock", "job", "run", value)
		ok, err := s.cache.Raw().SetNX(ctx, key, token, runLockTTL).Result()
		if err != nil {
			release()
			return nil, errcode.Wrap(errcode.ErrDatabase, err)
		}
		if !ok {
			release()
			return nil, errcode.Newf(errcode.ErrConflict, "任务或同一系统方法正在执行中")
		}
		keys = append(keys, key)
	}
	return release, nil
}
