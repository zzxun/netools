// +build etcd

package etcdc3

import (
	"context"
	"testing"
)

func TestClientPool_Client(t *testing.T) {
	pool, err := NewClientPool(&PoolConfig{
		Ctx: context.TODO(),
	})

	if err != nil {
		t.Error(err)
		return
	}

	ctx, cancel := context.WithTimeout(context.TODO(), etcdTimeout)
	defer cancel()
	_, err = pool.Select().Get(ctx, pool.TestKey)
	if err != nil {
		t.Error(err)
		return
	}
}
