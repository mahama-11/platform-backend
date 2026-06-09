#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import os
import re
import subprocess
import sys
import time
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any

ROOT = Path(__file__).resolve().parents[1]
REPORT_DIR = Path(os.environ.get("BUSINESS_CORRECTNESS_REPORT_DIR", str(ROOT / "reports" / "business-correctness"))).expanduser().resolve()
JSON_REPORT = REPORT_DIR / "business-correctness-gate-report.json"
MD_REPORT = REPORT_DIR / "business-correctness-gate-report.md"
DOC_PATH = ROOT / "docs" / "architecture" / "PLATFORM_BUSINESS_CORRECTNESS_ORACLE.md"


def display_path(path: Path) -> str:
    try:
        return str(path.relative_to(ROOT))
    except ValueError:
        return str(path)


@dataclass(frozen=True)
class CommandSpec:
    id: str
    argv: list[str]
    packages: list[str]
    expected_tests: list[str] = field(default_factory=list)
    timeout: int = 300
    report_path: str = "reports/business-correctness/business-correctness-gate-report.json"
    gate: str = "CI: .github/workflows/ci.yml / scripts/business-correctness-gate.py"


@dataclass(frozen=True)
class InvariantSpec:
    id: str
    statement: str
    critical: bool
    commands: list[str]
    journeys: list[str]
    metric: str


COMMANDS: dict[str, CommandSpec] = {
    "auth_org_access_journey": CommandSpec(
        id="auth_org_access_journey",
        packages=["./internal/modules/identity", "./internal/modules/access"],
        expected_tests=[
            "TestIdentityServiceRegisterLoginAndProfileFlow",
            "TestSeedDefaultsAndAccessContext",
            "TestHandlerMePermissionsAndInternalMembershipAccess",
            "TestIdentityServiceOwnerRoleDoesNotImplyPlatformAdmin",
            "TestIdentityServicePlatformAdminFlagGrantsPlatformPermission",
        ],
        argv=[
            "go", "test", "./internal/modules/identity", "./internal/modules/access",
            "-run",
            "^(TestIdentityServiceRegisterLoginAndProfileFlow|TestSeedDefaultsAndAccessContext|TestHandlerMePermissionsAndInternalMembershipAccess|TestIdentityServiceOwnerRoleDoesNotImplyPlatformAdmin|TestIdentityServicePlatformAdminFlagGrantsPlatformPermission)$",
            "-count=1",
        ],
    ),
    "menu_signup_package_activation_journey": CommandSpec(
        id="menu_signup_package_activation_journey",
        packages=["./internal/modules/control", "./internal/migration"],
        expected_tests=[
            "TestActivatePackageAppliesPoliciesAndIsIdempotent",
            "TestActivatePackageFailsClosedWithoutActivePolicies",
            "TestActivatePackageRejectsInactiveCommercialPackageWithoutPartialQuotaGrant",
            "TestSeedMenuOfferings",
        ],
        argv=[
            "go", "test", "./internal/modules/control", "./internal/migration",
            "-run",
            "^(TestActivatePackageAppliesPoliciesAndIsIdempotent|TestActivatePackageFailsClosedWithoutActivePolicies|TestActivatePackageRejectsInactiveCommercialPackageWithoutPartialQuotaGrant|TestSeedMenuOfferings)$",
            "-count=1",
        ],
    ),
    "quota_reserve_commit_release_journey": CommandSpec(
        id="quota_reserve_commit_release_journey",
        packages=["./internal/modules/control", "./internal/modules/metering", "./internal/modules/wallet"],
        expected_tests=[
            "TestControlServiceReserveCommitReleaseAndIdempotency",
            "TestReserveQuotaRejectsOverReservationAfterExistingHold",
            "TestFinalize_UsesReservationAndIsIdempotent",
            "TestFinalize_IdempotentRetryDoesNotReplaySettlementSideEffects",
            "TestIngestEvent_IncludedThenOverageConsumesQuotaBeforeBilling",
            "TestPostLedger_DebitIdempotencyWithReference",
            "TestPostLedger_ConcurrentDebitDoesNotOverdrawSharedSQLite",
        ],
        argv=[
            "go", "test", "./internal/modules/control", "./internal/modules/metering", "./internal/modules/wallet",
            "-run",
            "^(TestControlServiceReserveCommitReleaseAndIdempotency|TestReserveQuotaRejectsOverReservationAfterExistingHold|TestFinalize_UsesReservationAndIsIdempotent|TestFinalize_IdempotentRetryDoesNotReplaySettlementSideEffects|TestIngestEvent_IncludedThenOverageConsumesQuotaBeforeBilling|TestPostLedger_DebitIdempotencyWithReference|TestPostLedger_ConcurrentDebitDoesNotOverdrawSharedSQLite)$",
            "-count=1",
        ],
    ),
    "runtime_callback_asset_charge_journey": CommandSpec(
        id="runtime_callback_asset_charge_journey",
        packages=["./internal/modules/runtime", "./internal/modules/assetstorage", "./internal/modules/audit"],
        expected_tests=[
            "TestDispatchRuntimeJobAcceptsAsyncSubmissionAndEnqueuesPoll",
            "TestHandleProviderCallbackPayloadNormalizesOutputManifestAndRegistersStorage",
            "TestHandleProviderCallbackPayloadNormalizesTextInlineDataForStorage",
            "TestRuntimeTerminalChargeBindingCompletedSettlesReservedSessionIdempotently",
            "TestRuntimeTerminalChargeBindingFailedReleasesReservedSession",
            "TestRuntimeTerminalChargeBindingCanceledCancelsCreatedSession",
            "TestTransitionRuntimeJobStaleProviderEventsDoNotOverwriteTerminalDBRow",
            "TestHandleProviderCallbackPayloadProgressDoesNotDowngradeCompletedJob",
            "TestProductHTTPCallbackClientReturnsErrorOnHTTPFailure",
            "TestAuditServiceRecordAndHelpers",
        ],
        argv=[
            "go", "test", "./internal/modules/runtime", "./internal/modules/assetstorage", "./internal/modules/audit",
            "-run",
            "^(TestDispatchRuntimeJobAcceptsAsyncSubmissionAndEnqueuesPoll|TestHandleProviderCallbackPayloadNormalizesOutputManifestAndRegistersStorage|TestHandleProviderCallbackPayloadNormalizesTextInlineDataForStorage|TestRuntimeTerminalChargeBindingCompletedSettlesReservedSessionIdempotently|TestRuntimeTerminalChargeBindingFailedReleasesReservedSession|TestRuntimeTerminalChargeBindingCanceledCancelsCreatedSession|TestTransitionRuntimeJobStaleProviderEventsDoNotOverwriteTerminalDBRow|TestHandleProviderCallbackPayloadProgressDoesNotDowngradeCompletedJob|TestProductHTTPCallbackClientReturnsErrorOnHTTPFailure|TestAuditServiceRecordAndHelpers)$",
            "-count=1",
        ],
    ),
    "failure_path_journey": CommandSpec(
        id="failure_path_journey",
        packages=["./internal/modules/runtime", "./internal/modules/control", "./internal/modules/metering"],
        expected_tests=[
            "TestControlServiceErrorBranchesAndOptionalString",
            "TestControlHandlerReservationStateErrorsUseBusinessSemanticCodes",
            "TestHandleProviderCallbackValidatesSignatureAndReturnsTerminalJob",
            "TestEnqueueCallbackDeliveryFailsWhenProductEndpointMissing",
            "TestProductHTTPCallbackClientReturnsErrorOnHTTPFailure",
            "TestUpdateRuntimeJobRejectsFailedToQueued",
            "TestRuntimeTerminalChargeBindingRejectsBoundaryMismatchAndRollsBackJob",
            "TestMeteringHandlerSemanticErrorCodes",
            "TestMeteringListHandlersRejectMissingOrConflictingProductScope",
        ],
        argv=[
            "go", "test", "./internal/modules/runtime", "./internal/modules/control", "./internal/modules/metering",
            "-run",
            "^(TestControlServiceErrorBranchesAndOptionalString|TestControlHandlerReservationStateErrorsUseBusinessSemanticCodes|TestHandleProviderCallbackValidatesSignatureAndReturnsTerminalJob|TestEnqueueCallbackDeliveryFailsWhenProductEndpointMissing|TestProductHTTPCallbackClientReturnsErrorOnHTTPFailure|TestUpdateRuntimeJobRejectsFailedToQueued|TestRuntimeTerminalChargeBindingRejectsBoundaryMismatchAndRollsBackJob|TestMeteringHandlerSemanticErrorCodes|TestMeteringListHandlersRejectMissingOrConflictingProductScope)$",
            "-count=1",
        ],
    ),
    "contract_compatibility_compile": CommandSpec(
        id="contract_compatibility_compile",
        packages=["./..."],
        expected_tests=[],
        argv=["go", "test", "./...", "-run", "^$", "-count=1"],
        timeout=420,
    ),
}

INVARIANTS: list[InvariantSpec] = [
    InvariantSpec(
        id="INV-AUTH-001",
        statement="注册成功后必须有默认 active org context、owner membership、starter plan，登录/Me 能拿到同一 org 权限上下文。",
        critical=True,
        commands=["auth_org_access_journey"],
        journeys=["Platform auth/org/access journey"],
        metric="invariant_pass_rate",
    ),
    InvariantSpec(
        id="INV-MENU-SIGNUP-001",
        statement="Menu signup package 激活必须幂等；不能重复发额度；无 active policy/package 时 fail-closed 且不能留下部分 grant。",
        critical=True,
        commands=["menu_signup_package_activation_journey"],
        journeys=["Menu signup package activation journey"],
        metric="invariant_pass_rate",
    ),
    InvariantSpec(
        id="INV-PRODUCT-SCOPE-001",
        statement="product_code 缺失时不能返回跨产品钱包/计量/settlement 数据；显式 include_all_products 才能跨产品。",
        critical=True,
        commands=["failure_path_journey"],
        journeys=["失败链路：product scope missing/conflicting"],
        metric="invariant_pass_rate",
    ),
    InvariantSpec(
        id="INV-QUOTA-001",
        statement="quota reserve 成功但业务失败必须 release；commit/release/finalize 必须幂等，不能 over-reserve 或重复结算。",
        critical=True,
        commands=["quota_reserve_commit_release_journey", "failure_path_journey"],
        journeys=["Platform quota reserve/commit/release journey"],
        metric="charge_session_reconciliation",
    ),
    InvariantSpec(
        id="INV-RUNTIME-CHARGE-001",
        statement="runtime completed 必须 settle charge session；failed/canceled 必须 release/cancel charge session；boundary mismatch 必须回滚 runtime terminal update。",
        critical=True,
        commands=["runtime_callback_asset_charge_journey", "failure_path_journey"],
        journeys=["Runtime create → provider accepted → callback → result asset → charge settlement journey"],
        metric="runtime_job_terminal_distribution",
    ),
    InvariantSpec(
        id="INV-CALLBACK-001",
        statement="provider callback 必须校验签名；晚到 progress/callback 不能覆盖 terminal result；产品 endpoint 缺失或不可达必须失败可见。",
        critical=True,
        commands=["runtime_callback_asset_charge_journey", "failure_path_journey"],
        journeys=["失败链路：provider fail / duplicate callback / secret mismatch / product endpoint unreachable"],
        metric="callback_delivery_success_rate",
    ),
    InvariantSpec(
        id="INV-ASSET-CONSUMER-001",
        statement="provider result asset / inline output 必须规范化并注册到 storage，保证 product callback 可消费。",
        critical=True,
        commands=["runtime_callback_asset_charge_journey"],
        journeys=["Runtime callback → result asset → product consumer journey"],
        metric="contract_compatibility_pass_rate",
    ),
    InvariantSpec(
        id="INV-AUDIT-TRACE-001",
        statement="所有成本动作必须具备 metering / audit / request_id / trace_id 证据，不能产生无上下文的成本副作用。",
        critical=True,
        commands=["runtime_callback_asset_charge_journey", "quota_reserve_commit_release_journey"],
        journeys=["Cost action metering/audit/trace journey"],
        metric="quota_wallet_ledger_reconciliation_mismatch_count",
    ),
    InvariantSpec(
        id="INV-CONTRACT-001",
        statement="Platform 内部 API/DTO/consumer contract 必须编译兼容；破坏消费者时 gate fail-closed。",
        critical=True,
        commands=["contract_compatibility_compile"],
        journeys=["Contract compatibility journey"],
        metric="consumer_contract_break_count",
    ),
]

CRITICAL_JOURNEYS = {
    "Platform auth/org/access journey": ["auth_org_access_journey"],
    "Menu signup package activation journey": ["menu_signup_package_activation_journey"],
    "Platform quota reserve/commit/release journey": ["quota_reserve_commit_release_journey"],
    "Runtime create → provider accepted → callback → result asset → charge settlement journey": ["runtime_callback_asset_charge_journey"],
    "失败链路：余额不足、provider fail、重复 callback、secret mismatch、产品 endpoint 不可达": ["failure_path_journey"],
}


def run(argv: list[str], timeout: int) -> dict[str, Any]:
    started = time.time()
    try:
        proc = subprocess.run(
            argv,
            cwd=ROOT,
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            timeout=timeout,
        )
        return {
            "argv": argv,
            "exit_code": proc.returncode,
            "status": "PASS" if proc.returncode == 0 else "FAIL",
            "duration_seconds": round(time.time() - started, 3),
            "stdout_tail": proc.stdout[-8000:],
            "stderr_tail": proc.stderr[-8000:],
        }
    except subprocess.TimeoutExpired as exc:
        stdout = exc.stdout or ""
        stderr = exc.stderr or ""
        if isinstance(stdout, bytes):
            stdout = stdout.decode(errors="replace")
        if isinstance(stderr, bytes):
            stderr = stderr.decode(errors="replace")
        return {
            "argv": argv,
            "exit_code": 124,
            "status": "FAIL",
            "duration_seconds": round(time.time() - started, 3),
            "failure_reason": "timeout",
            "stdout_tail": stdout[-8000:],
            "stderr_tail": stderr[-8000:],
        }


def list_tests(package: str) -> dict[str, Any]:
    if package == "./...":
        return {"package": package, "status": "PASS", "tests": [], "exit_code": 0}
    result = run(["go", "test", package, "-list", "."], timeout=120)
    tests = sorted(line.strip() for line in result.get("stdout_tail", "").splitlines() if line.startswith("Test"))
    return {"package": package, **result, "tests": tests}


def preflight_command(spec: CommandSpec) -> dict[str, Any]:
    package_results = [list_tests(pkg) for pkg in spec.packages]
    present: set[str] = set()
    for item in package_results:
        present.update(item.get("tests", []))
    missing = sorted(set(spec.expected_tests) - present)
    failed_list = [item for item in package_results if item.get("exit_code") != 0]
    return {
        "status": "PASS" if not missing and not failed_list else "FAIL",
        "expected_tests": spec.expected_tests,
        "present_expected_tests": sorted(set(spec.expected_tests) & present),
        "missing_expected_tests": missing,
        "package_results": package_results,
    }


def maybe_run_prod_drift(args: argparse.Namespace) -> dict[str, Any]:
    if not args.run_prod_drift:
        return {
            "status": "NOT_RUN",
            "critical_count": None,
            "warning_count": None,
            "blocking": bool(args.require_prod_drift),
            "note": "Set --run-prod-drift --require-prod-drift for live prod correctness evidence after approved prod repair.",
        }
    drift_args = ["tools/prod/platform-drift-check.sh", "--env", "prod", "--fail-on-critical"]
    result = run(drift_args, timeout=360)
    output = (result.get("stdout_tail") or "") + "\n" + (result.get("stderr_tail") or "")
    summary = re.search(r"SUMMARY\s+critical=(\d+)\s+warnings=(\d+)", output)
    critical = int(summary.group(1)) if summary else None
    warnings = int(summary.group(2)) if summary else None
    return {
        **result,
        "critical_count": critical,
        "warning_count": warnings,
        "blocking": bool(args.require_prod_drift),
        "evidence_command": drift_args,
    }


def doc_traceability_check() -> dict[str, Any]:
    if not DOC_PATH.exists():
        return {"status": "FAIL", "missing_file": str(DOC_PATH), "missing_invariant_ids": [inv.id for inv in INVARIANTS]}
    text = DOC_PATH.read_text(encoding="utf-8")
    missing_ids = [inv.id for inv in INVARIANTS if inv.id not in text]
    required_terms = [
        "business invariant -> Test* / smoke / SelfCheck command -> report path -> CI gate",
        "invariant pass rate",
        "critical journey pass rate",
        "prod drift critical count",
        "consumer contract break count",
    ]
    missing_terms = [term for term in required_terms if term not in text]
    return {
        "status": "PASS" if not missing_ids and not missing_terms else "FAIL",
        "doc_path": str(DOC_PATH.relative_to(ROOT)),
        "missing_invariant_ids": missing_ids,
        "missing_required_terms": missing_terms,
    }


def pct(passed: int, total: int) -> float:
    return round((passed / total * 100.0) if total else 100.0, 2)


def write_markdown(payload: dict[str, Any]) -> None:
    lines = [
        "# Platform Business Correctness Gate Report",
        "",
        f"- status: **{payload['status']}**",
        f"- generated_at: `{payload['generated_at']}`",
        f"- invariant_pass_rate: `{payload['metrics']['invariant_pass_rate']}%`",
        f"- critical_journey_pass_rate: `{payload['metrics']['critical_journey_pass_rate']}%`",
        f"- contract_compatibility_pass_rate: `{payload['metrics']['contract_compatibility_pass_rate']}%`",
        f"- prod_drift_critical_count: `{payload['metrics']['prod_drift_critical_count']}`",
        "",
        "## Invariant Traceability",
        "",
        "| invariant | status | executable evidence | journeys |",
        "|---|---:|---|---|",
    ]
    for row in payload["invariant_results"]:
        cmd_text = ", ".join(row["commands"])
        journey_text = "; ".join(row["journeys"])
        lines.append(f"| {row['id']} | {row['status']} | {cmd_text} | {journey_text} |")
    lines.extend([
        "",
        "## Command Results",
        "",
        "| command | status | argv |",
        "|---|---:|---|",
    ])
    for cid, item in payload["command_results"].items():
        lines.append(f"| {cid} | {item['status']} | `{' '.join(item['argv'])}` |")
    MD_REPORT.write_text("\n".join(lines) + "\n", encoding="utf-8")


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="Platform business correctness oracle gate")
    parser.add_argument("--run-prod-drift", action="store_true", help="Run read-only prod drift check and include it in metrics")
    parser.add_argument("--require-prod-drift", action="store_true", help="Fail if prod drift evidence is not PASS with critical_count=0")
    args = parser.parse_args(argv)

    started = time.time()
    REPORT_DIR.mkdir(parents=True, exist_ok=True)

    preflight_results = {cid: preflight_command(spec) for cid, spec in COMMANDS.items()}
    command_results: dict[str, Any] = {}
    for cid, spec in COMMANDS.items():
        if preflight_results[cid]["status"] != "PASS":
            command_results[cid] = {
                "id": cid,
                "status": "FAIL",
                "argv": spec.argv,
                "failure_reason": "expected tests missing or package list failed",
                "preflight": preflight_results[cid],
            }
            continue
        command_results[cid] = {"id": cid, **run(spec.argv, timeout=spec.timeout), "preflight": preflight_results[cid]}

    invariant_results = []
    for inv in INVARIANTS:
        cmd_statuses = [command_results[cid]["status"] for cid in inv.commands]
        status = "PASS" if all(s == "PASS" for s in cmd_statuses) else "FAIL"
        invariant_results.append({
            "id": inv.id,
            "statement": inv.statement,
            "critical": inv.critical,
            "status": status,
            "commands": inv.commands,
            "journeys": inv.journeys,
            "metric": inv.metric,
        })

    journey_results = []
    for journey, cids in CRITICAL_JOURNEYS.items():
        status = "PASS" if all(command_results[cid]["status"] == "PASS" for cid in cids) else "FAIL"
        journey_results.append({"journey": journey, "status": status, "commands": cids})

    doc_check = doc_traceability_check()
    prod_drift = maybe_run_prod_drift(args)

    inv_pass = sum(1 for item in invariant_results if item["status"] == "PASS")
    journey_pass = sum(1 for item in journey_results if item["status"] == "PASS")
    contract_pass = 1 if command_results["contract_compatibility_compile"]["status"] == "PASS" else 0
    prod_drift_ok = (
        prod_drift["status"] == "PASS" and prod_drift.get("critical_count") == 0
    )
    prod_drift_blocker = bool(args.require_prod_drift) and not prod_drift_ok

    failures = [cid for cid, item in command_results.items() if item["status"] != "PASS"]
    status = "PASS" if not failures and doc_check["status"] == "PASS" and not prod_drift_blocker else "FAIL"

    payload = {
        "feature": "platform-business-correctness-oracle",
        "verifier": "business-correctness-gate",
        "status": status,
        "generated_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "duration_seconds": round(time.time() - started, 3),
        "policy": "Correctness is gated by executable business invariants and critical journeys, not raw coverage alone. Any missing expected test, failed command, missing traceability doc, or required live prod drift critical finding is FAIL.",
        "metrics": {
            "invariant_pass_rate": pct(inv_pass, len(invariant_results)),
            "critical_journey_pass_rate": pct(journey_pass, len(journey_results)),
            "contract_compatibility_pass_rate": pct(contract_pass, 1),
            "prod_drift_critical_count": prod_drift.get("critical_count"),
            "runtime_job_terminal_success_failure_distribution": {
                "source": "targeted local Go tests",
                "completed_settled": command_results["runtime_callback_asset_charge_journey"]["status"],
                "failed_released": command_results["runtime_callback_asset_charge_journey"]["status"],
                "canceled_canceled": command_results["runtime_callback_asset_charge_journey"]["status"],
                "live_prod_distribution": "NOT_RUN" if not args.run_prod_drift else "covered_by_prod_drift_topology_only; runtime smoke is separate",
            },
            "charge_session_settled_released_canceled_reconciliation": {
                "source": "targeted local Go tests",
                "mismatch_count": 0 if command_results["runtime_callback_asset_charge_journey"]["status"] == "PASS" else None,
            },
            "callback_delivery_success_rate": {
                "source": "targeted callback success/failure Go tests",
                "pass_rate": 100.0 if command_results["runtime_callback_asset_charge_journey"]["status"] == "PASS" and command_results["failure_path_journey"]["status"] == "PASS" else 0.0,
                "live_rate": "NOT_RUN" if not args.run_prod_drift else "not derived from drift check",
            },
            "quota_wallet_ledger_reconciliation_mismatch_count": 0 if command_results["quota_reserve_commit_release_journey"]["status"] == "PASS" else None,
            "consumer_contract_break_count": 0 if contract_pass else 1,
        },
        "invariants": [inv.__dict__ for inv in INVARIANTS],
        "invariant_results": invariant_results,
        "critical_journey_results": journey_results,
        "preflight_results": preflight_results,
        "command_results": command_results,
        "doc_traceability_check": doc_check,
        "prod_drift_evidence": prod_drift,
        "failed_command_ids": failures,
        "report_paths": {"json": display_path(JSON_REPORT), "markdown": display_path(MD_REPORT)},
    }
    JSON_REPORT.write_text(json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    write_markdown(payload)
    print(json.dumps({
        "status": status,
        "metrics": payload["metrics"],
        "failed_command_ids": failures,
        "json_report": str(JSON_REPORT),
        "markdown_report": str(MD_REPORT),
    }, ensure_ascii=False, indent=2))
    print(f"SELF_CHECK_EVIDENCE: {JSON_REPORT}")
    return 0 if status == "PASS" else 2


if __name__ == "__main__":
    raise SystemExit(main())
