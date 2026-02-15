package controller

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/resource"
)

// testQty parses a quantity string; panics on error (test only).
func testQty(s string) resource.Quantity {
	return resource.MustParse(s)
}

// testQtyBytes builds a quantity in binary bytes.
func testQtyBytes(n int64) resource.Quantity {
	return *resource.NewQuantity(n, resource.BinarySI)
}

// annotThreshold returns a single annotation map for the memory threshold key.
func annotThreshold(val string) map[string]string {
	if val == "" {
		return nil
	}

	return map[string]string{PreoomkillerAnnotationMemoryThresholdKey: val}
}

// ptrQty returns a pointer to the quantity (for memoryLimit fields).
func ptrQty(q resource.Quantity) *resource.Quantity {
	return &q
}

// newTestPod builds a Pod with fixed name/namespace for tests.
func newTestPod(annotations map[string]string, memoryLimit *resource.Quantity) Pod {
	return Pod{
		Name:        "test-pod",
		Namespace:   "default",
		Annotations: annotations,
		MemoryLimit: memoryLimit,
	}
}

// resolveCase is one row of the resolveMemoryThreshold table.
type resolveCase struct {
	name        string
	annotations map[string]string
	memoryLimit *resource.Quantity
	wantErr     error
	wantQty     resource.Quantity
}

func Test_resolveMemoryThreshold(t *testing.T) {
	t.Parallel()

	logger := slog.Default()

	tests := []resolveCase{
		// Error cases
		{
			name:        "missing annotation",
			annotations: nil,
			memoryLimit: ptrQty(testQty("1Gi")),
			wantErr:     ErrMemoryThresholdParse,
		},
		{
			name:        "absolute invalid",
			annotations: annotThreshold("not-a-quantity"),
			memoryLimit: ptrQty(testQty("1Gi")),
			wantErr:     ErrMemoryThresholdParse,
		},
		{
			name:        "percentage no limit",
			annotations: annotThreshold("80%"),
			memoryLimit: nil,
			wantErr:     ErrMemoryLimitNotDefined,
		},
		{
			name:        "percentage zero limit",
			annotations: annotThreshold("80%"),
			memoryLimit: ptrQty(testQtyBytes(0)),
			wantErr:     ErrMemoryLimitNotDefined,
		},
		{
			name:        "percentage invalid number",
			annotations: annotThreshold("x%"),
			memoryLimit: ptrQty(testQty("1Gi")),
			wantErr:     ErrMemoryThresholdParse,
		},
		{
			name:        "percentage out of range 0",
			annotations: annotThreshold("0%"),
			memoryLimit: ptrQty(testQty("1Gi")),
			wantErr:     ErrMemoryThresholdParse,
		},
		{
			name:        "percentage out of range 101",
			annotations: annotThreshold("101%"),
			memoryLimit: ptrQty(testQty("1Gi")),
			wantErr:     ErrMemoryThresholdParse,
		},
		// Absolute threshold
		{
			name:        "absolute valid",
			annotations: annotThreshold("512Mi"),
			memoryLimit: ptrQty(testQty("1Gi")),
			wantQty:     testQty("512Mi"),
		},
		// Percentage threshold
		{
			name:        "percentage valid 80",
			annotations: annotThreshold("80%"),
			memoryLimit: ptrQty(testQty("1Gi")),
			// 80% of 1Gi (1073741824) = 858993459
			wantQty: testQtyBytes(858993459),
		},
		{
			name:        "percentage with spaces 50",
			annotations: annotThreshold(" 50 %"),
			memoryLimit: ptrQty(testQty("1Gi")),
			// 50% of 1Gi = 536870912
			wantQty: testQtyBytes(536870912),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pod := newTestPod(tt.annotations, tt.memoryLimit)

			got, err := resolveMemoryThreshold(t.Context(), logger, pod)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)

				return
			}

			require.NoError(t, err)
			require.True(t, tt.wantQty.Equal(got), "quantity: got %s, want %s", got.String(), tt.wantQty.String())
		})
	}
}
