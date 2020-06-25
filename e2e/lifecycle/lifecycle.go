package lifecycle

import (
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

type LifecycleE2ETest struct {
	framework.TC
	jobIDs []string
}

func init() {
	framework.AddSuites(&framework.TestSuite{
		Component:   "Lifecycle",
		CanRunLocal: true,
		Cases:       []framework.TestCase{new(LifecycleE2ETest)},
	})
}

// Ensure cluster has leader and at least 1 client node
// in a ready state before running tests
func (tc *LifecycleE2ETest) BeforeAll(f *framework.F) {
	e2eutil.WaitForLeader(f.T(), tc.Nomad())
	e2eutil.WaitForNodesReady(f.T(), tc.Nomad(), 1)
}

// TestBatchJob runs a batch job with prestart and poststop hooks
func (tc *LifecycleE2ETest) TestBatchJob(f *framework.F) {
	t := f.T()
	require := require.New(t)
	nomadClient := tc.Nomad()
	uuid := uuid.Generate()
	jobID := "lifecycle-" + uuid[0:8]
	tc.jobIDs = append(tc.jobIDs, jobID)

	allocs := e2eutil.RegisterAndWaitForAllocs(f.T(), nomadClient, "lifecycle/inputs/batch.nomad", jobID, "")
	require.Equal(1, len(allocs))
	allocID := allocs[0].ID

	// wait for the job to stop and assert we stopped successfully, not failed
	e2eutil.WaitForAllocStopped(t, nomadClient, allocID)
	alloc, _, err := nomadClient.Allocations().Info(allocID, nil)
	require.NoError(err)
	require.Equal(structs.AllocClientStatusComplete, alloc.ClientStatus)

	// assert the files were written as expected
	afi, _, err := nomadClient.AllocFS().List(alloc, "alloc", nil)
	require.NoError(err)
	expected := map[string]bool{
		"init-ran": true, "main-ran": true, "cleanup-ran": true,
		"init-running": false, "main-running": false, "cleanup-running": false}
	got := checkFiles(expected, afi)
	require.Equal(expected, got)
}

// TestServiceJob runs a service job with prestart and poststop hooks
func (tc *LifecycleE2ETest) TestServiceJob(f *framework.F) {
	t := f.T()
	require := require.New(t)
	nomadClient := tc.Nomad()
	uuid := uuid.Generate()
	jobID := "lifecycle-" + uuid[0:8]
	tc.jobIDs = append(tc.jobIDs, jobID)

	allocs := e2eutil.RegisterAndWaitForAllocs(f.T(), nomadClient, "lifecycle/inputs/service.nomad", jobID, "")
	require.Equal(1, len(allocs))
	allocID := allocs[0].ID

	e2eutil.WaitForAllocRunning(t, nomadClient, allocID)

	alloc, _, err := nomadClient.Allocations().Info(allocID, nil)
	require.NoError(err)

	// stop the job
	_, _, err = nomadClient.Jobs().Deregister(jobID, false, nil)
	require.NoError(err)
	e2eutil.WaitForAllocStopped(t, nomadClient, allocID)

	// assert the files were written as expected
	afi, _, err := nomadClient.AllocFS().List(alloc, "alloc", nil)
	require.NoError(err)
	expected := map[string]bool{
		"init-ran": true, "sidecar-ran": true, "main-ran": true, "cleanup-ran": true,
		"init-running": false, "sidecar-running": false,
		"main-running": false, "cleanup-running": false}
	got := checkFiles(expected, afi)
	require.Equal(expected, got)
}

// checkFiles returns a map of whether the expected files were found
// in the file info response
func checkFiles(expected map[string]bool, got []*api.AllocFileInfo) map[string]bool {
	results := map[string]bool{}
	for expect := range expected {
		results[expect] = false
	}
	for _, file := range got {
		// there will be files unrelated to the test, so ignore those
		if _, ok := results[file.Name]; ok {
			results[file.Name] = true
		}
	}
	return results
}
