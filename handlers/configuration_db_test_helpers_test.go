package handlers

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type fakeConfigurationQueries struct {
	rows         pgx.Rows
	queryErr     error
	rowResults   []fakeConfigurationRowResult
	queryCalls   int
	rowCalls     int
	lastQuery    string
	lastRowQuery string
	lastRowArgs  []any
	rowQueries   []string
}

type fakeConfigurationRowResult struct {
	values []any
	err    error
}

func (f *fakeConfigurationQueries) Query(_ context.Context, query string, _ ...any) (pgx.Rows, error) {
	f.queryCalls++
	f.lastQuery = query
	return f.rows, f.queryErr
}

func (f *fakeConfigurationQueries) QueryRow(_ context.Context, query string, args ...any) pgx.Row {
	f.rowCalls++
	f.lastRowQuery = query
	f.lastRowArgs = args
	f.rowQueries = append(f.rowQueries, query)
	if len(f.rowResults) == 0 {
		return fakeConfigurationRow{err: errors.New("unexpected QueryRow call")}
	}
	result := f.rowResults[0]
	f.rowResults = f.rowResults[1:]
	return fakeConfigurationRow(result)
}

type fakeConfigurationRow fakeConfigurationRowResult

func (r fakeConfigurationRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	return scanConfigurationValues(r.values, dest)
}

type fakeConfigurationRows struct {
	values  [][]any
	index   int
	current []any
	err     error
	scanErr error
	closed  bool
}

func (r *fakeConfigurationRows) Close()                                       { r.closed = true }
func (r *fakeConfigurationRows) Err() error                                   { return r.err }
func (r *fakeConfigurationRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *fakeConfigurationRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *fakeConfigurationRows) Next() bool {
	if r.index >= len(r.values) {
		return false
	}
	r.current = r.values[r.index]
	r.index++
	return true
}
func (r *fakeConfigurationRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	return scanConfigurationValues(r.current, dest)
}
func (r *fakeConfigurationRows) Values() ([]any, error) { return r.current, nil }
func (r *fakeConfigurationRows) RawValues() [][]byte    { return nil }
func (r *fakeConfigurationRows) Conn() *pgx.Conn        { return nil }

func scanConfigurationValues(values []any, dest []any) error {
	if len(values) != len(dest) {
		return fmt.Errorf("fake row has %d values, scan requested %d destinations", len(values), len(dest))
	}
	for i := range values {
		target := reflect.ValueOf(dest[i])
		if target.Kind() != reflect.Pointer || target.IsNil() {
			return fmt.Errorf("destination %d is not a non-nil pointer", i)
		}
		value := target.Elem()
		if values[i] == nil {
			value.Set(reflect.Zero(value.Type()))
			continue
		}
		source := reflect.ValueOf(values[i])
		if source.Type().AssignableTo(value.Type()) {
			value.Set(source)
			continue
		}
		if source.Type().ConvertibleTo(value.Type()) {
			value.Set(source.Convert(value.Type()))
			continue
		}
		return fmt.Errorf("cannot scan fake value %T into %s", values[i], value.Type())
	}
	return nil
}
