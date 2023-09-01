package tests

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/fredyk/westack-go/westack/datasource"
)

func Test_Datasource_Initialize_InvalidDatasource(t *testing.T) {

	t.Parallel()

	ds := datasource.New("test2020", app.DsViper, context.Background())
	err := ds.Initialize()
	assert.Error(t, err)
	assert.Regexp(t, "invalid connector", err.Error(), "error message should be 'invalid connector'")

}

func Test_Datasource_Initialize_ConnectError(t *testing.T) {

	t.Parallel()

	prevHost := app.DsViper.GetString("db.host")
	ds := datasource.New("db0", app.DsViper, context.Background())
	ds.SubViper.Set("host", "<invalid host>")
	ds.Options = &datasource.Options{
		MongoDB: &datasource.MongoDBDatasourceOptions{
			Timeout: 3,
		},
	}
	err := ds.Initialize()
	assert.Error(t, err)
	assert.Regexp(t, "no such host", err.Error(), "error message should be 'no such host'")

	ds.SubViper.Set("host", prevHost)
	err = ds.Initialize()
	assert.NoError(t, err)

}

func Test_DatasourceClose(t *testing.T) {

	t.Parallel()

	ds, err := app.FindDatasource("db_expected_to_be_closed")
	assert.NoError(t, err)
	assert.NotNil(t, ds)

	err = ds.Close()
	assert.NoError(t, err)

	// Based on suggestion from @tanryberdi: https://github.com/fredyk/westack-go/pull/480#discussion_r1312634782
	// Attempt to perform a query. We don't mind the queried collection because the client
	// is disconnected anyway
	result, err := ds.FindMany("unknownCollection", nil)
	assert.Errorf(t, err, "client is disconnected")
	assert.Nil(t, result)

}

func Test_Datasource_Ping(t *testing.T) {

	t.Parallel()

	// Simply wait 3.2 seconds to cover datasource ping interval
	time.Sleep(3200 * time.Millisecond)

}

func Test_Datasource_Ping_Will_Fail(t *testing.T) {

	t.Parallel()

	db, err := app.FindDatasource("db_expected_to_fail")
	assert.NoError(t, err)
	assert.NotNil(t, db)

	// Wait 0.1 seconds, then change host and expect to fail
	time.Sleep(100 * time.Millisecond)

	db.SetTimeout(0.1)

	// Wait 5.1 seconds to cover datasource ping interval
	time.Sleep(5100 * time.Millisecond)

}
