// == comparison_test.go ==

package box

import (
	"sync/atomic"
	"testing"
	"time"
)

// Подход 1: Наивный подход с any (GC будет утилизировать)
func BenchmarkComparison_Single_NaiveAny(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		msg := nativeProducer(i)
		if err := nativeConsumer(msg); err != nil {
			b.Fatal(err)
		}
	}
}

// Подход 2: Отдельный пул для каждого типа сообщений
func BenchmarkComparison_Single_DedicatedPools(b *testing.B) {
	dp := NewDedicatedPools()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		msg := poolsProducer(i, dp)
		if err := poolsConsumer(msg, dp); err != nil {
			b.Fatal(err)
		}
	}
}

// Подход 3: Наш Box подход (для сравнения)
func BenchmarkComparison_Single_Box(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		msg := boxProducer(i)
		if err := boxConsumer(msg.Transfer()); err != nil {
			b.Fatal(err)
		}
	}
}

// Конкурентные версии всех подходов
func BenchmarkComparison_Concurrent_NaiveAny(b *testing.B) {
	b.ReportAllocs()

	var counter int64
	start := time.Now()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			i := int(atomic.AddInt64(&counter, 1) - 1)
			msg := nativeProducer(i)
			if err := nativeConsumer(msg); err != nil {
				b.Fatal(err)
			}
		}
	})

	elapsed := time.Since(start)
	rps := float64(b.N) / elapsed.Seconds()
	b.ReportMetric(rps, "rps")
}

func BenchmarkComparison_Concurrent_DedicatedPools(b *testing.B) {
	dp := NewDedicatedPools()
	b.ReportAllocs()

	var counter int64
	start := time.Now()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			i := int(atomic.AddInt64(&counter, 1) - 1)
			msg := poolsProducer(i, dp)
			if err := poolsConsumer(msg, dp); err != nil {
				b.Fatal(err)
			}
		}
	})

	elapsed := time.Since(start)
	rps := float64(b.N) / elapsed.Seconds()
	b.ReportMetric(rps, "rps")
}

func BenchmarkComparison_Concurrent_Box(b *testing.B) {
	b.ReportAllocs()

	var counter int64
	start := time.Now()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			i := int(atomic.AddInt64(&counter, 1) - 1)
			msg := boxProducer(i)
			if err := boxConsumer(msg); err != nil {
				b.Fatal(err)
			}
		}
	})

	elapsed := time.Since(start)
	rps := float64(b.N) / elapsed.Seconds()
	b.ReportMetric(rps, "rps")
}
