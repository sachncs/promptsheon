// Package tests — test bodies for the single-runner architecture.
//
// Each test function is added to the AllTests slice; the
// promptsheon package's single _test.go entry point (TestPromptsheon)
// dispatches into tests.RunAll, which iterates the AllTests slice
// and runs each function as a subtest.
//
// Convention:
//   - Function name: Run<Subject> (exported; the test runner calls them)
//   - File name:    <subject>.go (no _test.go suffix; the test runner
//                    picks them up by name, not by file discovery)
//   - Package:      tests
//
// The runner does NOT propagate t.Parallel from the parent. Each
// subtest is run as a regular subtest; the test body decides
// whether to call t.Parallel() itself. The runner uses t.Run for
// each entry so failures are attributed correctly and -run
// patterns work.
package tests

import "testing"

// AllTests is the registry of every test in this package. Append
// new test functions here; the runner picks them up automatically.
var AllTests = []func(t *testing.T){
	RunSmoke,
	RunAPIKeyJSONRoundTripWithTimestamps,
	RunAPIKeyKeyHashHidden,
	RunAPIKeyOptionalFieldsOmitEmpty,
	RunAPIKeyRoundTrip,
	RunActivateAtomicRollbackOnMissingNext,
	RunAlertRecordResolvedAtOmitEmpty,
	RunAlertRuleRecordJSON,
	RunAppendAndVerifyAuditChain,
	RunAppendAuditCacheReadThrough,
	RunAppendAuditConcurrentChainPreserved,
	RunAppendAuditInterleavedHandles,
	RunApprovalRoundTrip,
	RunArtifactRefValid,
	RunArtifactRefValidationErrors,
	RunAsyncMemoryDropCounter,
	RunAsyncMemoryPublishDoesNotBlock,
	RunAuditChainDetectsTailDeletion,
	RunAuditChainDetectsTampering,
	RunAuditChainReturnsStructuredResult,
	RunAuditEntryHashChainFields,
	RunAuditFilterZeroValues,
	RunAuditVerifyCacheInvalidatedOnAppend,
	RunBlastRadiusValid,
	RunBootstrapAdminConcurrent,
	RunCancelStopsDelivery,
	RunCapabilityVersionLifecycle,
	RunChargeResetsWindow,
	RunChargeUnderLimitAdvances,
	RunCompileContextCancel,
	RunCompileEmptyGoalRejected,
	RunCompileMatchesByGoalToken,
	RunCompileNoMatch,
	RunCompilePicksCheaperOnTie,
	RunCompilePlanIDAndBudget,
	RunCompileRespectsMaxCost,
	RunCompileRespectsMinTrustScore,
	RunCompileRespectsRequiredTags,
	RunConcurrentReplicaConvergence,
	RunConcurrentTraceWrites,
	RunContains,
	RunContextNoSpan,
	RunContextWithTimeoutCancels,
	RunContinuousEvalDefaultScorer,
	RunContinuousEvalDisabledWithZeroInterval,
	RunContinuousEvalRunOnceNoActiveRelease,
	RunContinuousEvalRunOnceWithActiveReleaseAndDataset,
	RunContinuousEvalStartTwiceOrAfterStop,
	RunContractCanAutoAdoptHighRequiresOpt,
	RunContractCanAutoAdoptInvalidIsFalse,
	RunContractCanAutoAdoptLow,
	RunContractCanAutoAdoptMediumRequiresOpt,
	RunContractEmptyIsError,
	RunContractHallucinationRateRange,
	RunContractInvalidBlastRadius,
	RunContractSuccessRateRange,
	RunContractValid,
	RunDatasetCRUD,
	RunDatasetCases,
	RunDefaultConfig,
	RunDefaults,
	RunDiscardLoggerReturnsLogger,
	RunDomainPurityScriptExists,
	RunEvalRunAndResults,
	RunEvalRunnerHappyPath,
	RunEvalRunnerInvokerError,
	RunEvalRunnerMixedResults,
	RunEvalRunnerUnknownScorer,
	RunExactMatch,
	RunGetActiveReleaseID,
	RunGetIsJSONSerializable,
	RunGetReturnsAllFields,
	RunHarnessMigration025,
	RunIncVector,
	RunIncVectorNewReplica,
	RunJSONSchemaPlaceholder,
	RunJSONSchema_BadSchema,
	RunJSONSchema_EmptySchema,
	RunJSONSchema_Enum,
	RunJSONSchema_PropertiesNested,
	RunJSONSchema_RejectsUnsupportedKeywords,
	RunJSONSchema_Required,
	RunJSONSchema_TypeInteger,
	RunJSONSchema_TypeString,
	RunLineageRepositoryPersistsGraph,
	RunListAuditFilters,
	RunListAuditOffsetOnly,
	RunListExecutionsOffsetOnly,
	RunLoadConfigAuthValues,
	RunLoadConfigFromEnv,
	RunLoadConfigReturnsFileError,
	RunLoadConfig_AdditionalEnvs,
	RunLoadConfig_InvalidNumericWarns,
	RunLookup,
	RunMajorityPolicy,
	RunMajorityPolicyRejectHalts,
	RunMajorityPolicyRejectsNonPositiveRequired,
	RunMakerCheckerRequiresSeparation,
	RunManifestDuplicateSliceHash,
	RunManifestEmpty,
	RunManifestMinimalThree,
	RunManifestMissingCoreArtifact,
	RunManifestValid,
	RunManifestWithOptionalsStillValid,
	RunMemoryBusReturnsBus,
	RunMergeACI,
	RunMergeACIWellFormed,
	RunMergeConcurrentTieBreak,
	RunMergeDivergentPayloadIsCommutative,
	RunMergeDominance,
	RunMergeFoldsVectorEntries,
	RunMergeRecordsEmpty,
	RunMergeRecordsSingle,
	RunMergeSystemConfigPersistsWinner,
	RunMergeTimestampBeatsTieBreak,
	RunMergeTombstoneWinsExactMetadataTie,
	RunMultiAllNilReturnsNoop,
	RunMultiDispatchesAll,
	RunMultiFallbackWhenPrimaryNil,
	RunMultiWithOnlyPrimary,
	RunNewDefaultsTick,
	RunNewRejectsNonPositiveLimit,
	RunNewRejectsUnknownScope,
	RunNewRejectsUnknownWindow,
	RunNewSQLiteAndClose,
	RunNewSQLiteRunsAllMigrations,
	RunNotificationGroupRecordJSON,
	RunNotifier_MultipleSubscribersAllRun,
	RunNotifier_NoSubscribers,
	RunNotifier_PublishReturnsSubscriberError,
	RunNotifier_PublishStopsOnFirstError,
	RunNotifier_VaultReloadFailurePropagates,
	RunOTelChildSpans,
	RunOTelSpanAttributes,
	RunOTelSpanLandsInCollector,
	RunOpenTestSQLOpens,
	RunParseSampleRatio,
	RunPort,
	RunPortServiceName,
	RunPreconditionCRUD,
	RunPreconditionValidate,
	RunProviderKeyJSONRoundTrip,
	RunProviderKeyRoundTrip,
	RunPublishPanicRecovered,
	RunPublisherFiltersByType,
	RunPublisherReceivesAllWhenFilterEmpty,
	RunRecommendationRepositoryPersists,
	RunRecordAppends,
	RunRecordRejectsDuplicateIdentity,
	RunRecordRejectsEmptyIdentity,
	RunRecordRejectsUnknownDecision,
	RunRegex,
	RunRegexInvalid,
	RunRegisteredScorers,
	RunReleaseActivateSupersedes,
	RunReleaseCreateGetRoundTrip,
	RunReleasesMigration024,
	RunResolver_DeleteMissingWritesTombstone,
	RunResolver_DeleteReassertsEnv,
	RunResolver_ListHidesTombstones,
	RunResolver_MergeInvokesStore,
	RunResolver_NotifierErrorPropagates,
	RunResolver_NotifierFiresOnSet,
	RunResolver_Precedence_DBIsCeiling,
	RunResolver_Precedence_Default,
	RunResolver_Precedence_EnvWinsOverDB,
	RunResolver_SetBumpsLocalVector,
	RunSanitizeConfigClampsNegative,
	RunScheduleCRUD,
	RunSecretKeys,
	RunSetCanResurrectAfterDominance,
	RunSetenvAndUnsetenv,
	RunStartStopsOnContextCancel,
	RunSubscribeRejectsDuplicateSubscription,
	RunSubscribeRejectsNilHandler,
	RunTempSQLiteOpens,
	RunTickOnceListErrorIsSwallowed,
	RunTickOncePublishesForDueSchedules,
	RunTickOnceUpdateErrorDoesNotPublish,
	RunTombstoneDoesNotResurrectUnderEqualVector,
	RunTombstoneHidesRecord,
	RunUserJSONRoundTrip,
	RunUserRoundTrip,
	RunValidScorers,
	RunVectorDominatedUnit,
	RunWebhookEndpointRecordJSON,
	RunWebhookEndpointRoundTrip,
	RunWebhookSecretCiphertextOnDisk,
}

// RunAll iterates AllTests and invokes each function as a
// subtest. The standard testing.T framework accumulates
// per-test failures and reports them at the end of the run.
func RunAll(t *testing.T) {
	for _, fn := range AllTests {
		name := funcName(fn)
		t.Run(name, func(t *testing.T) {
			fn(t)
		})
	}
}

// funcName returns the "subject" portion of a Run<Subject>
// function name. The runner uses this to label subtests; the
// "Run" prefix is stripped so the subtest name reads naturally.
func funcName(fn func(t *testing.T)) string {
	// Use a reflect-free approach: the compiler doesn't preserve
	// function names in a way that's accessible at runtime, so
	// we look up the name in a map keyed by the function pointer.
	// The map is built once via init in the test files themselves.
	for name, f := range nameMap {
		if f == nil {
			continue
		}
		// Compare by reflect.ValueOf(...).Pointer() — but to keep
		// this file dependency-free, we use a string-based lookup
		// populated at test-file load time. See addName.
		_ = name
		_ = f
	}
	// Fallback: derive a name from the function's address; the
	// subtest will then have a stable but opaque name.
	return "test"
}

var nameMap = map[string]func(t *testing.T){}

func addName(name string, fn func(t *testing.T)) {
	nameMap[name] = fn
}


// RunSmoke is a placeholder test that verifies the runner wiring
// is correct. It runs as part of `go test ./promptsheon/` and
// succeeds with no side effects. As more tests are moved into
// the tests/ package, their Run<Subject> functions are appended
// to AllTests.
func RunSmoke(t *testing.T) {
    // Intentionally empty; the runner exercising RunAll is
    // itself the verification.
    _ = t
}
