package metrics

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDatadogProvider_RunQuery(t *testing.T) {
	appKey := "app-key"
	apiKey := "api-key"
	t.Run("ok", func(t *testing.T) {
		expected := 1.11111
		eq := `avg:system.cpu.user{*}by{host}`
		now := time.Now().Unix()
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			aq := r.URL.Query().Get("query")
			assert.Equal(t, eq, aq)
			assert.Equal(t, appKey, r.Header.Get(datadogApplicationKeyHeaderKey))
			assert.Equal(t, apiKey, r.Header.Get(datadogAPIKeyHeaderKey))

			from, err := strconv.ParseInt(r.URL.Query().Get("from"), 10, 64)
			if assert.NoError(t, err) {
				assert.Less(t, from, now)
			}

			to, err := strconv.ParseInt(r.URL.Query().Get("to"), 10, 64)
			if assert.NoError(t, err) {
				assert.GreaterOrEqual(t, to, now)
			}

			json := fmt.Sprintf(`{"series": [{"pointlist": [[1577232000000,29325.102158814265],[1577318400000,56294.46758591842],[1577404800000,%f]]}]}`, expected)
			w.Write([]byte(json))
		}))
		defer ts.Close()

		dp, err := NewDatadogProvider(
			ProviderOpts{Address: ts.URL, Interval: 1 * time.Minute},
			DatadogOpts{
				ApplicationKey: appKey,
				APIKey:         apiKey,
			},
		)
		require.NoError(t, err)

		f, err := dp.Execute(eq)
		require.NoError(t, err)
		assert.Equal(t, expected, f)
	})

	t.Run("no values", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json := fmt.Sprintf(`{"series": [{"pointlist": []}]}`)
			w.Write([]byte(json))
		}))
		defer ts.Close()

		dp, err := NewDatadogProvider(ProviderOpts{Address: ts.URL, Interval: 1 * time.Minute},
			DatadogOpts{
				ApplicationKey: appKey,
				APIKey:         apiKey,
			},
		)
		require.NoError(t, err)
		_, err = dp.Execute("")
		require.True(t, errors.Is(err, ErrNoValuesFound))
	})
}
