package schemadiff

type ImpactLevel int

const (
	LevelUnset ImpactLevel = 0
	Level1     ImpactLevel = 1 // minimal impact
	Level2     ImpactLevel = 2 // moderate impact
	Level3     ImpactLevel = 3 // significant impact
)

func (l ImpactLevel) String() string {
	switch l {
	case Level3:
		return "LEVEL_3"
	case Level2:
		return "LEVEL_2"
	case Level1:
		return "LEVEL_1"
	default:
		return "UNKNOWN"
	}
}

type classificationKey struct {
	ObjectType ObjectType
	DiffType   DiffType
	Property   string
}

var impactMap = map[classificationKey]ImpactLevel{
	// Tables
	{ObjectTable, DiffDropped, ""}:              Level3,
	{ObjectTable, DiffAdded, ""}:                Level2,
	{ObjectTable, DiffModified, "IS_PARTITIONED"}:   Level3,
	{ObjectTable, DiffModified, "PARTITION_STRATEGY"}: Level3,
	{ObjectTable, DiffModified, "REPLICA_IDENTITY"}:   Level2,

	// Columns
	{ObjectColumn, DiffDropped, ""}:             Level3,
	{ObjectColumn, DiffAdded, ""}:               Level2,
	{ObjectColumn, DiffModified, "TYPE_CHANGED"}:     Level3,
	{ObjectColumn, DiffModified, "NULLABLE_CHANGED"}: Level2,
	{ObjectColumn, DiffModified, "DEFAULT_CHANGED"}:  Level1,
	{ObjectColumn, DiffModified, "IDENTITY_CHANGED"}: Level2,

	// Constraints — severity depends on constraint type, handled in classifyConstraint
	{ObjectConstraint, DiffDropped, "p"}: Level3, // PK dropped
	{ObjectConstraint, DiffDropped, "u"}: Level2, // UK dropped
	{ObjectConstraint, DiffDropped, "f"}: Level1, // FK dropped
	{ObjectConstraint, DiffDropped, "c"}: Level1, // CHECK dropped
	{ObjectConstraint, DiffDropped, "x"}: Level1, // Exclusion dropped
	{ObjectConstraint, DiffAdded, ""}:    Level1,
	{ObjectConstraint, DiffModified, "COLUMNS_CHANGED"}:    Level3,
	{ObjectConstraint, DiffModified, "TYPE_CHANGED"}:       Level3,
	{ObjectConstraint, DiffModified, "EXPRESSION_CHANGED"}: Level1,

	// Indexes
	{ObjectIndex, DiffDropped, ""}:                 Level1,
	{ObjectIndex, DiffAdded, ""}:                   Level1,
	{ObjectIndex, DiffModified, "METHOD_CHANGED"}:      Level1,
	{ObjectIndex, DiffModified, "COLUMNS_CHANGED"}:     Level1,
	{ObjectIndex, DiffModified, "UNIQUENESS_CHANGED"}:  Level1,
	{ObjectIndex, DiffModified, "WHERE_CHANGED"}:       Level1,

	// Sequences
	{ObjectSequence, DiffDropped, ""}:              Level1,
	{ObjectSequence, DiffAdded, ""}:                Level1,
	{ObjectSequence, DiffModified, "TYPE_CHANGED"}:      Level1,
	{ObjectSequence, DiffModified, "INCREMENT_CHANGED"}: Level1,

	// Views / MVs
	{ObjectView, DiffDropped, ""}:                  Level1,
	{ObjectView, DiffAdded, ""}:                    Level1,
	{ObjectView, DiffModified, "DEFINITION_CHANGED"}:   Level1,
	{ObjectMView, DiffDropped, ""}:                 Level1,
	{ObjectMView, DiffAdded, ""}:                   Level1,
	{ObjectMView, DiffModified, "DEFINITION_CHANGED"}:  Level1,

	// Functions
	{ObjectFunction, DiffDropped, ""}:                  Level1,
	{ObjectFunction, DiffAdded, ""}:                    Level1,
	{ObjectFunction, DiffModified, "RETURN_TYPE_CHANGED"}:  Level1,
	{ObjectFunction, DiffModified, "LANGUAGE_CHANGED"}:     Level1,
	{ObjectFunction, DiffModified, "VOLATILITY_CHANGED"}:   Level1,

	// Triggers
	{ObjectTrigger, DiffDropped, ""}:                  Level2,
	{ObjectTrigger, DiffAdded, ""}:                    Level2,
	{ObjectTrigger, DiffModified, "FUNCTION_CHANGED"}:     Level2,
	{ObjectTrigger, DiffModified, "DEFINITION_CHANGED"}:   Level2,
	{ObjectTrigger, DiffModified, "ENABLED_CHANGED"}:      Level2,

	// Enum types
	{ObjectEnumType, DiffDropped, ""}:              Level2,
	{ObjectEnumType, DiffAdded, ""}:                Level1,
	{ObjectEnumType, DiffModified, "VALUES_CHANGED"}:   Level2,

	// Domain types
	{ObjectDomainType, DiffDropped, ""}:             Level2,
	{ObjectDomainType, DiffAdded, ""}:               Level1,
	{ObjectDomainType, DiffModified, "BASE_TYPE_CHANGED"}: Level2,

	// Extensions
	{ObjectExtension, DiffDropped, ""}:              Level1,
	{ObjectExtension, DiffAdded, ""}:                Level1,
	{ObjectExtension, DiffModified, "VERSION_CHANGED"}:  Level1,

	// Policies
	{ObjectPolicy, DiffDropped, ""}:                 Level1,
	{ObjectPolicy, DiffAdded, ""}:                   Level1,
	{ObjectPolicy, DiffModified, "EXPRESSION_CHANGED"}: Level1,
}

func ClassifyImpact(entry DiffEntry) ImpactLevel {
	// For constraints, use the constraint type stored in Property for dropped entries
	if entry.ObjectType == ObjectConstraint && entry.DiffType == DiffDropped {
		key := classificationKey{entry.ObjectType, entry.DiffType, entry.Property}
		if level, ok := impactMap[key]; ok {
			return level
		}
	}

	key := classificationKey{entry.ObjectType, entry.DiffType, entry.Property}
	if level, ok := impactMap[key]; ok {
		return level
	}

	// Fallback: ADDED/DROPPED without specific entry → use generic
	key = classificationKey{entry.ObjectType, entry.DiffType, ""}
	if level, ok := impactMap[key]; ok {
		return level
	}

	return Level1
}

func classifyAll(result *DiffResult) {
	for i := range result.Entries {
		result.Entries[i].ImpactLevel = ClassifyImpact(result.Entries[i])
	}
}
