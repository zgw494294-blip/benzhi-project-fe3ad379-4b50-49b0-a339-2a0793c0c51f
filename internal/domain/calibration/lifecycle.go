package calibration

type Capabilities struct {
	RegisterSensor  bool `json:"registerSensor"`
	LockProfile     bool `json:"lockProfile"`
	SubmitReading   bool `json:"submitReading"`
	Recalibrate     bool `json:"recalibrate"`
	ReturnReview    bool `json:"returnReview"`
	ApproveReview   bool `json:"approveReview"`
	ResubmitReview  bool `json:"resubmitReview"`
	IssueCredential bool `json:"issueCredential"`
	QueryOnly       bool `json:"queryOnly"`
}

var legalTransitions = map[BatchStatus]map[BatchStatus]bool{
	StatusDraft: {
		StatusPlanLocked: true,
	},
	StatusPlanLocked: {
		StatusSampling:    true,
		StatusFailed:      true,
		StatusReadyReview: true,
	},
	StatusSampling: {
		StatusSampling:    true,
		StatusFailed:      true,
		StatusReadyReview: true,
	},
	StatusFailed: {
		StatusSampling: true,
		StatusReturned: true,
	},
	StatusReadyReview: {
		StatusReturned: true,
		StatusFrozen:   true,
	},
	StatusReturned: {
		StatusReturned:    true,
		StatusReadyReview: true,
	},
	StatusFrozen: {
		StatusReleased: true,
	},
	StatusReleased: {},
}

func AllowedTransition(from, to BatchStatus) bool {
	next, known := legalTransitions[from]
	return known && next[to]
}

func (b *CalibrationBatch) MoveTo(next BatchStatus) error {
	if !AllowedTransition(b.Status, next) {
		return Conflict("批次不能从 %s 转换到 %s", b.Status, next)
	}
	b.Status = next
	b.Version++
	return nil
}

func (b *CalibrationBatch) Capabilities() Capabilities {
	capabilities := Capabilities{}
	switch b.Status {
	case StatusDraft:
		capabilities.RegisterSensor = true
		capabilities.LockProfile = true
	case StatusPlanLocked, StatusSampling:
		capabilities.SubmitReading = true
	case StatusFailed:
		capabilities.SubmitReading = true
		capabilities.Recalibrate = true
		capabilities.ReturnReview = true
	case StatusReadyReview:
		capabilities.ReturnReview = true
		capabilities.ApproveReview = true
	case StatusReturned:
		capabilities.SubmitReading = true
		capabilities.Recalibrate = true
		capabilities.ResubmitReview = true
	case StatusFrozen:
		capabilities.IssueCredential = true
	case StatusReleased:
		capabilities.QueryOnly = true
	}
	return capabilities
}

func (b *CalibrationBatch) RequireCapability(name string) error {
	capabilities := b.Capabilities()
	allowed := false
	switch name {
	case "register_sensor":
		allowed = capabilities.RegisterSensor
	case "lock_profile":
		allowed = capabilities.LockProfile
	case "submit_reading":
		allowed = capabilities.SubmitReading
	case "recalibrate":
		allowed = capabilities.Recalibrate
	case "return_review":
		allowed = capabilities.ReturnReview
	case "approve_review":
		allowed = capabilities.ApproveReview
	case "issue_credential":
		allowed = capabilities.IssueCredential
	default:
		return Validation("未知领域能力 %s", name)
	}
	if !allowed {
		return Conflict("批次状态 %s 不允许操作 %s", b.Status, name)
	}
	return nil
}
