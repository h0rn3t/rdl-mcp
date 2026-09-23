package rdl

import (
	"regexp"
	"slices"
	"strings"
)

var (
	fieldReferencePattern = regexp.MustCompile(`Fields!([\pL\pN_]+)`)
	lookupPattern         = regexp.MustCompile(`(?i)(Lookup|LookupSet|MultiLookup)\s*\(\s*([^,]+),\s*([^,]+),\s*([^,]+),\s*"([^"]+)"\s*\)`)
	aggregatePattern      = regexp.MustCompile(`(?i)(Sum|Count|First|Last|Min|Max|Avg|CountDistinct|StDev|StDevP|Var|VarP|CountRows|RunningValue|Previous)\s*\(\s*([^,)]+?)(?:\s*,\s*"([^"]+)")?\s*\)`)
)

// ExtractFieldsWithContext groups field references by their dataset scope.
func ExtractFieldsWithContext(expression, defaultDataset string) map[string][]string {
	result := make(map[string][]string)
	if !strings.HasPrefix(strings.TrimSpace(expression), "=") {
		return result
	}
	handled := make(map[int]struct{})
	for _, match := range lookupPattern.FindAllStringSubmatchIndex(expression, -1) {
		target := expression[match[10]:match[11]]
		for _, part := range []struct {
			group   int
			dataset string
		}{
			{2, defaultDataset}, {3, target}, {4, target},
		} {
			start := match[2*part.group]
			end := match[2*part.group+1]
			for _, field := range fieldReferencePattern.FindAllStringSubmatchIndex(expression[start:end], -1) {
				handled[start+field[0]] = struct{}{}
				appendField(result, part.dataset, expression[start+field[2]:start+field[3]])
			}
		}
	}
	for _, match := range aggregatePattern.FindAllStringSubmatchIndex(expression, -1) {
		dataset := defaultDataset
		if match[6] >= 0 {
			dataset = expression[match[6]:match[7]]
		}
		start, end := match[4], match[5]
		for _, field := range fieldReferencePattern.FindAllStringSubmatchIndex(expression[start:end], -1) {
			position := start + field[0]
			if _, seen := handled[position]; seen {
				continue
			}
			handled[position] = struct{}{}
			appendField(result, dataset, expression[start+field[2]:start+field[3]])
		}
	}
	for _, field := range fieldReferencePattern.FindAllStringSubmatchIndex(expression, -1) {
		if _, seen := handled[field[0]]; seen {
			continue
		}
		appendField(result, defaultDataset, expression[field[2]:field[3]])
	}
	return result
}

// ExtractFields returns the unique field names in an expression.
func ExtractFields(expression string) []string {
	result := make([]string, 0)
	if !strings.HasPrefix(strings.TrimSpace(expression), "=") {
		return result
	}
	for _, match := range fieldReferencePattern.FindAllStringSubmatch(expression, -1) {
		if !slices.Contains(result, match[1]) {
			result = append(result, match[1])
		}
	}
	slices.Sort(result)
	return result
}

func appendField(result map[string][]string, dataset, field string) {
	if !slices.Contains(result[dataset], field) {
		result[dataset] = append(result[dataset], field)
	}
}
