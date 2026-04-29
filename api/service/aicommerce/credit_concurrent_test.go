package aicommerce_test

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
)

// simulateDeduct 模拟带 CAS 的积分扣减逻辑（不依赖数据库）
// 等价于 UPDATE users SET power = power - amount WHERE id = ? AND power >= amount
func simulateDeduct(balance *int64, amount int64) bool {
	for {
		cur := atomic.LoadInt64(balance)
		if cur < amount {
			return false // 积分不足
		}
		if atomic.CompareAndSwapInt64(balance, cur, cur-amount) {
			return true
		}
		// CAS 失败，重试
	}
}

// TestConcurrentCreditDeduction 验证并发场景下积分扣减的原子性
// 确保最终余额不会为负，且成功次数 * cost <= 初始余额
func TestConcurrentCreditDeduction(t *testing.T) {
	const (
		initialBalance int64 = 100
		costPerTask    int64 = 10
		goroutines           = 50 // 远超可承受的并发量
	)

	var balance int64 = initialBalance
	var successCount int64

	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if simulateDeduct(&balance, costPerTask) {
				atomic.AddInt64(&successCount, 1)
			}
		}()
	}
	wg.Wait()

	finalBalance := atomic.LoadInt64(&balance)
	finalSuccess := atomic.LoadInt64(&successCount)

	// 余额不能为负
	if finalBalance < 0 {
		t.Errorf("balance went negative: %d", finalBalance)
	}

	// 成功次数 * cost 应该等于消耗量
	expectedConsumed := finalSuccess * costPerTask
	actualConsumed := initialBalance - finalBalance
	if expectedConsumed != actualConsumed {
		t.Errorf("consumed mismatch: success=%d * cost=%d = %d, but balance dropped by %d",
			finalSuccess, costPerTask, expectedConsumed, actualConsumed)
	}

	// 最多只能成功 initialBalance/cost 次
	maxSuccess := initialBalance / costPerTask
	if finalSuccess > maxSuccess {
		t.Errorf("too many successes: %d > max %d", finalSuccess, maxSuccess)
	}

	t.Logf("goroutines=%d, success=%d/%d, balance=%d→%d",
		goroutines, finalSuccess, maxSuccess, initialBalance, finalBalance)
}

// TestCreditRefund 验证退款逻辑正确性
func TestCreditRefund(t *testing.T) {
	var balance int64 = 50

	// 模拟扣减
	if !simulateDeduct(&balance, 30) {
		t.Fatal("expected deduct to succeed")
	}
	if balance != 20 {
		t.Errorf("after deduct: balance=%d, want 20", balance)
	}

	// 模拟退款（原子加）
	atomic.AddInt64(&balance, 30)
	if balance != 50 {
		t.Errorf("after refund: balance=%d, want 50", balance)
	}
}

// TestInsufficientCredit 验证积分不足时拒绝扣减
func TestInsufficientCredit(t *testing.T) {
	cases := []struct {
		balance int64
		cost    int64
		want    bool
	}{
		{10, 10, true},
		{9, 10, false},
		{0, 1, false},
		{100, 101, false},
	}
	for _, c := range cases {
		b := c.balance
		got := simulateDeduct(&b, c.cost)
		if got != c.want {
			t.Errorf("deduct(balance=%d, cost=%d): got %v, want %v", c.balance, c.cost, got, c.want)
		}
		if got && b != c.balance-c.cost {
			t.Errorf("balance after deduct: got %d, want %d", b, c.balance-c.cost)
		}
	}
	_ = fmt.Sprintf // suppress unused import
}
