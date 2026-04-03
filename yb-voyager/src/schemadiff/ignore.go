package schemadiff

type IgnoreRule struct {
	Name   string
	Match  func(d DiffEntry) bool
	Reason string
}

type FilterResult struct {
	Real       []DiffEntry
	Suppressed []SuppressedEntry
}

type SuppressedEntry struct {
	Entry  DiffEntry
	Reason string
}

func FilterDiff(entries []DiffEntry, rules []IgnoreRule) *FilterResult {
	result := &FilterResult{}
	for _, entry := range entries {
		suppressed := false
		for _, rule := range rules {
			if rule.Match(entry) {
				result.Suppressed = append(result.Suppressed, SuppressedEntry{
					Entry:  entry,
					Reason: rule.Reason,
				})
				suppressed = true
				break
			}
		}
		if !suppressed {
			result.Real = append(result.Real, entry)
		}
	}
	return result
}

// BtreeToLSMRule suppresses btree→lsm index method differences.
var BtreeToLSMRule = IgnoreRule{
	Name: "btree-to-lsm",
	Match: func(d DiffEntry) bool {
		return d.ObjectType == ObjectIndex &&
			d.DiffType == DiffModified &&
			d.Property == "METHOD_CHANGED" &&
			d.OldValue == "btree" &&
			d.NewValue == "lsm"
	},
	Reason: "YugabyteDB uses LSM storage for all btree indexes.",
}

// RedundantIndexRule suppresses indexes removed by Voyager as redundant.
// The caller must provide the set of redundant index names (from redundant_indexes.sql).
func RedundantIndexRule(redundantIndexNames map[string]bool) IgnoreRule {
	return IgnoreRule{
		Name: "redundant-index-removed",
		Match: func(d DiffEntry) bool {
			return d.ObjectType == ObjectIndex &&
				d.DiffType == DiffDropped &&
				redundantIndexNames[d.ObjectName]
		},
		Reason: "Removed by Voyager as redundant (covered by a stronger index). Original saved in redundant_indexes.sql.",
	}
}

// PendingPostDataImportRule suppresses objects deferred until finalize-schema-post-data-import.
// The caller must provide the set of deferred object names.
func PendingPostDataImportRule(deferredObjects map[string]bool) IgnoreRule {
	return IgnoreRule{
		Name: "pending-post-data-import",
		Match: func(d DiffEntry) bool {
			if d.DiffType != DiffDropped {
				return false
			}
			if d.ObjectType != ObjectIndex && d.ObjectType != ObjectConstraint {
				return false
			}
			return deferredObjects[d.ObjectName]
		},
		Reason: "Deferred until 'finalize-schema-post-data-import'.",
	}
}

// UnsupportedGiSTIndexRule suppresses GiST indexes missing on YB target.
var UnsupportedGiSTIndexRule = IgnoreRule{
	Name: "yb-unsupported-gist",
	Match: func(d DiffEntry) bool {
		return d.ObjectType == ObjectIndex &&
			d.DiffType == DiffDropped &&
			false // TODO: needs access method info from the dropped index
	},
	Reason: "GiST indexes are not supported on YugabyteDB.",
}

func DefaultPGvsYBRules(redundantIndexNames, deferredObjects map[string]bool) []IgnoreRule {
	rules := []IgnoreRule{
		BtreeToLSMRule,
	}
	if len(redundantIndexNames) > 0 {
		rules = append(rules, RedundantIndexRule(redundantIndexNames))
	}
	if len(deferredObjects) > 0 {
		rules = append(rules, PendingPostDataImportRule(deferredObjects))
	}
	return rules
}
