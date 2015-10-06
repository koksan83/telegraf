package lustre2

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/koksan83/telegraf/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Set config file variables to point to fake directory structure instead of /proc?

const obdfilterProcContents = `snapshot_time             1438693064.430544 secs.usecs
read_bytes                203238095 samples [bytes] 4096 1048576 78026117632000
write_bytes               71893382 samples [bytes] 1 1048576 15201500833981
get_info                  1182008495 samples [reqs]
set_info_async            2 samples [reqs]
connect                   1117 samples [reqs]
reconnect                 1160 samples [reqs]
disconnect                1084 samples [reqs]
statfs                    3575885 samples [reqs]
create                    698 samples [reqs]
destroy                   3190060 samples [reqs]
setattr                   605647 samples [reqs]
punch                     805187 samples [reqs]
sync                      6608753 samples [reqs]
preprw                    275131477 samples [reqs]
commitrw                  275131477 samples [reqs]
quotactl                  229231 samples [reqs]
ping                      78020757 samples [reqs]
`

const osdldiskfsProcContents = `snapshot_time             1438693135.640551 secs.usecs
get_page                  275132812 samples [usec] 0 3147 1320420955 22041662259
cache_access              19047063027 samples [pages] 1 1 19047063027
cache_hit                 7393729777 samples [pages] 1 1 7393729777
cache_miss                11653333250 samples [pages] 1 1 11653333250
`

const mdtProcContents = `snapshot_time             1438693238.20113 secs.usecs
open                      1024577037 samples [reqs]
close                     873243496 samples [reqs]
mknod                     349042 samples [reqs]
link                      445 samples [reqs]
unlink                    3549417 samples [reqs]
mkdir                     705499 samples [reqs]
rmdir                     227434 samples [reqs]
rename                    629196 samples [reqs]
getattr                   1503663097 samples [reqs]
setattr                   1898364 samples [reqs]
getxattr                  6145349681 samples [reqs]
setxattr                  83969 samples [reqs]
statfs                    2916320 samples [reqs]
sync                      434081 samples [reqs]
samedir_rename            259625 samples [reqs]
crossdir_rename           369571 samples [reqs]
`

type metrics struct {
	name  string
	value uint64
}

func TestLustre2GeneratesMetrics(t *testing.T) {

	tempdir := os.TempDir() + "/telegraf/rootfs/proc/fs/lustre/"
	ost_name := "OST0001"

	mdtdir := tempdir + "/mdt/"
	err := os.MkdirAll(mdtdir+"/"+ost_name, 0755)
	require.NoError(t, err)

	osddir := tempdir + "/osd-ldiskfs/"
	err = os.MkdirAll(osddir+"/"+ost_name, 0755)
	require.NoError(t, err)

	obddir := tempdir + "/obdfilter/"
	err = os.MkdirAll(obddir+"/"+ost_name, 0755)
	require.NoError(t, err)

	err = ioutil.WriteFile(mdtdir+"/"+ost_name+"/md_stats", []byte(mdtProcContents), 0644)
	require.NoError(t, err)

	err = ioutil.WriteFile(osddir+"/"+ost_name+"/stats", []byte(osdldiskfsProcContents), 0644)
	require.NoError(t, err)

	err = ioutil.WriteFile(obddir+"/"+ost_name+"/stats", []byte(obdfilterProcContents), 0644)
	require.NoError(t, err)

	m := &Lustre2{
		Ost_procfiles: []string{obddir + "/*/stats", osddir + "/*/stats"},
		Mds_procfiles: []string{mdtdir + "/*/md_stats"},
	}

	var acc testutil.Accumulator

	err = m.Gather(&acc)
	require.NoError(t, err)

	tags := map[string]string{
		"name": ost_name,
	}

	intMetrics := []*metrics{
		{
			name:  "write_bytes",
			value: 15201500833981,
		},
		{
			name:  "read_bytes",
			value: 78026117632000,
		},
		{
			name:  "write_calls",
			value: 71893382,
		},
		{
			name:  "read_calls",
			value: 203238095,
		},
		{
			name:  "cache_hit",
			value: 7393729777,
		},
		{
			name:  "cache_access",
			value: 19047063027,
		},
		{
			name:  "cache_miss",
			value: 11653333250,
		},
	}

	for _, metric := range intMetrics {
		assert.True(t, acc.HasUIntValue(metric.name), metric.name)
		assert.True(t, acc.CheckTaggedValue(metric.name, metric.value, tags))
	}

	err = os.RemoveAll(os.TempDir() + "/telegraf")
	require.NoError(t, err)
}
