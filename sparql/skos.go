package query

import (
	"net/url"
	"strings"

	"github.com/soypete/ontology-go/types"
)

const (
	SKOSNS = "http://www.w3.org/2004/02/skos/core#"

	SKOSConcept           = SKOSNS + "Concept"
	SKOSConceptScheme     = SKOSNS + "ConceptScheme"
	SKOSCollection        = SKOSNS + "Collection"
	SKOSOrderedCollection = SKOSNS + "OrderedCollection"

	SKOSPrefLabel   = SKOSNS + "prefLabel"
	SKOSAltLabel    = SKOSNS + "altLabel"
	SKOSHiddenLabel = SKOSNS + "hiddenLabel"
	SKOSNotation    = SKOSNS + "notation"

	SKOSNote          = SKOSNS + "note"
	SKOSChangeNote    = SKOSNS + "changeNote"
	SKOSDefinition    = SKOSNS + "definition"
	SKOSEditorialNote = SKOSNS + "editorialNote"
	SKOSExample       = SKOSNS + "example"
	SKOSHistoryNote   = SKOSNS + "historyNote"
	SKOSScopeNote     = SKOSNS + "scopeNote"

	SKOSBroader            = SKOSNS + "broader"
	SKOSNarrower           = SKOSNS + "narrower"
	SKOSRelated            = SKOSNS + "related"
	SKOSBroaderTransitive  = SKOSNS + "broaderTransitive"
	SKOSNarrowerTransitive = SKOSNS + "narrowerTransitive"
	SKOSSemanticRelation   = SKOSNS + "semanticRelation"

	SKOSBroadMatch      = SKOSNS + "broadMatch"
	SKOSNarrowMatch     = SKOSNS + "narrowMatch"
	SKOSRelatedMatch    = SKOSNS + "relatedMatch"
	SKOSExactMatch      = SKOSNS + "exactMatch"
	SKOSCloseMatch      = SKOSNS + "closeMatch"
	SKOSMappingRelation = SKOSNS + "mappingRelation"

	SKOSInScheme      = SKOSNS + "inScheme"
	SKOSHasTopConcept = SKOSNS + "hasTopConcept"
	SKOSTopConceptOf  = SKOSNS + "topConceptOf"

	SKOSMember     = SKOSNS + "member"
	SKOSMemberList = SKOSNS + "memberList"
)

type SKOSInferenceOption int

const (
	SKOSInferenceNone SKOSInferenceOption = iota
	SKOSInferenceBroader
	SKOSInferenceNarrower
	SKOSInferenceRelated
	SKOSInferenceExactMatch
	SKOSInferenceCloseMatch
	SKOSInferenceAll
)

type AuthorityMatchMode int

const (
	AuthorityMatchNone AuthorityMatchMode = iota
	AuthorityMatchFull
	AuthorityMatchAuthority
)

type SKOSOptions struct {
	Inference     SKOSInferenceOption
	AuthorityMode AuthorityMatchMode
}

var defaultSKOSOptions = SKOSOptions{
	Inference:     SKOSInferenceNone,
	AuthorityMode: AuthorityMatchNone,
}

type EngineOption func(*Engine)

func WithSKOSInference(opt SKOSInferenceOption) EngineOption {
	return func(e *Engine) {
		e.skosOptions.Inference = opt
	}
}

func WithAuthorityMatch(mode AuthorityMatchMode) EngineOption {
	return func(e *Engine) {
		e.skosOptions.AuthorityMode = mode
	}
}

func (e *Engine) SetSKOSOptions(opts SKOSOptions) {
	e.skosOptions = opts
}

func (e *Engine) GetSKOSOptions() SKOSOptions {
	return e.skosOptions
}

func (e *Engine) inferSKOSTriples(triples []types.Triple) []types.Triple {
	if e.skosOptions.Inference == SKOSInferenceNone {
		return triples
	}

	var inferred []types.Triple
	inferred = append(inferred, triples...)

	inference := e.skosOptions.Inference

	if inference == SKOSInferenceAll || inference == SKOSInferenceBroader {
		inferred = append(inferred, e.inferBroaderNarrower(triples, false)...)
	}

	if inference == SKOSInferenceAll || inference == SKOSInferenceNarrower {
		inferred = append(inferred, e.inferBroaderNarrower(triples, true)...)
	}

	if inference == SKOSInferenceAll || inference == SKOSInferenceRelated {
		inferred = append(inferred, e.inferRelated(triples)...)
	}

	if inference == SKOSInferenceAll || inference == SKOSInferenceExactMatch {
		inferred = append(inferred, e.inferExactMatch(triples)...)
	}

	if inference == SKOSInferenceAll || inference == SKOSInferenceCloseMatch {
		inferred = append(inferred, e.inferCloseMatch(triples)...)
	}

	if e.skosOptions.AuthorityMode != AuthorityMatchNone {
		inferred = e.addAuthorityTriples(inferred)
	}

	return inferred
}

func (e *Engine) inferBroaderNarrower(triples []types.Triple, inverse bool) []types.Triple {
	var inferred []types.Triple

	broaderPred := SKOSBroader
	narrowerPred := SKOSNarrower
	if inverse {
		broaderPred, narrowerPred = narrowerPred, broaderPred
	}

	graphMap := make(map[string]map[string][]string)
	for _, t := range triples {
		if t.Predicate == broaderPred {
			inferred = append(inferred, types.Triple{
				Subject:   t.Subject,
				Predicate: narrowerPred,
				Object:    t.Object,
				Graph:     t.Graph,
			})
			if _, ok := graphMap[t.Graph]; !ok {
				graphMap[t.Graph] = make(map[string][]string)
			}
			graphMap[t.Graph][t.Subject] = append(graphMap[t.Graph][t.Subject], t.Object)
		}
	}

	for graph, subjectMap := range graphMap {
		inferred = append(inferred, e.computeTransitiveBroader(subjectMap, broaderPred, narrowerPred, graph)...)
	}

	return inferred
}

func (e *Engine) computeTransitiveBroader(subjectMap map[string][]string, broaderPred, narrowerPred, graph string) []types.Triple {
	var inferred []types.Triple

	var dfs func(originalSubject, current string, path map[string]bool)

	visitedPairs := make(map[string]bool)

	dfs = func(originalSubject, current string, path map[string]bool) {
		if path[current] {
			return
		}
		path[current] = true
		defer delete(path, current)

		for _, broader := range subjectMap[current] {
			pairKey := originalSubject + "|" + broader
			if !visitedPairs[pairKey] {
				visitedPairs[pairKey] = true
				inferred = append(inferred, types.Triple{
					Subject:   originalSubject,
					Predicate: broaderPred,
					Object:    broader,
					Graph:     graph,
				})
				dfs(originalSubject, broader, path)
			}
		}
	}

	for subject := range subjectMap {
		dfs(subject, subject, make(map[string]bool))
	}

	return inferred
}

func (e *Engine) inferRelated(triples []types.Triple) []types.Triple {
	var inferred []types.Triple

	relatedPairs := make(map[string]map[string]bool)

	for _, t := range triples {
		if t.Predicate == SKOSRelated {
			if relatedPairs[t.Subject] == nil {
				relatedPairs[t.Subject] = make(map[string]bool)
			}
			relatedPairs[t.Subject][t.Object] = true
		}
	}

	for subject, objects := range relatedPairs {
		for obj := range objects {
			inferred = append(inferred, types.Triple{
				Subject:   obj,
				Predicate: SKOSRelated,
				Object:    subject,
				Graph:     "",
			})
		}
	}

	return inferred
}

func (e *Engine) inferExactMatch(triples []types.Triple) []types.Triple {
	var inferred []types.Triple

	matchMap := make(map[string][]string)
	allConcepts := make(map[string]struct{})

	for _, t := range triples {
		if t.Predicate == SKOSExactMatch {
			matchMap[t.Subject] = append(matchMap[t.Subject], t.Object)
			allConcepts[t.Subject] = struct{}{}
			allConcepts[t.Object] = struct{}{}
		}
	}

	visitedPairs := make(map[string]bool)
	var transitiveInfer func(subject string, visited map[string]bool)

	transitiveInfer = func(subject string, visited map[string]bool) {
		if visited[subject] {
			return
		}
		visited[subject] = true

		for _, directMatch := range matchMap[subject] {
			for otherConcept := range allConcepts {
				if otherConcept == subject || otherConcept == directMatch {
					continue
				}
				pairKey := directMatch + "|" + otherConcept
				if !visitedPairs[pairKey] {
					visitedPairs[pairKey] = true
					inferred = append(inferred, types.Triple{
						Subject:   directMatch,
						Predicate: SKOSExactMatch,
						Object:    otherConcept,
						Graph:     "",
					})
					inferred = append(inferred, types.Triple{
						Subject:   otherConcept,
						Predicate: SKOSExactMatch,
						Object:    directMatch,
						Graph:     "",
					})
				}
			}
			transitiveInfer(directMatch, visited)
		}
	}

	for subject := range matchMap {
		transitiveInfer(subject, make(map[string]bool))
	}

	return inferred
}

func (e *Engine) inferCloseMatch(triples []types.Triple) []types.Triple {
	var inferred []types.Triple

	directMatches := make(map[string][]string)
	resourceToConcepts := make(map[string][]string)

	for _, t := range triples {
		if t.Predicate == SKOSCloseMatch {
			directMatches[t.Object] = append(directMatches[t.Object], t.Subject)
			resourceToConcepts[t.Object] = append(resourceToConcepts[t.Object], t.Subject)
		}
	}

	for obj, subjects := range directMatches {
		for _, sub := range subjects {
			inferred = append(inferred, types.Triple{
				Subject:   sub,
				Predicate: SKOSExactMatch,
				Object:    obj,
				Graph:     "",
			})
			inferred = append(inferred, types.Triple{
				Subject:   obj,
				Predicate: SKOSExactMatch,
				Object:    sub,
				Graph:     "",
			})
		}
	}

	for resource, concepts := range resourceToConcepts {
		if len(concepts) < 2 {
			continue
		}
		for i := 0; i < len(concepts); i++ {
			for j := i + 1; j < len(concepts); j++ {
				inferred = append(inferred, types.Triple{
					Subject:   concepts[i],
					Predicate: SKOSExactMatch,
					Object:    concepts[j],
					Graph:     "",
				})
				inferred = append(inferred, types.Triple{
					Subject:   concepts[j],
					Predicate: SKOSExactMatch,
					Object:    concepts[i],
					Graph:     "",
				})
			}
		}
		_ = resource
	}

	return inferred
}

func (e *Engine) addAuthorityTriples(triples []types.Triple) []types.Triple {
	var result []types.Triple
	result = append(result, triples...)

	authorityIndex := make(map[string][]int)

	for i, t := range triples {
		if isHTTPURI(t.Subject) {
			auth := getAuthority(t.Subject)
			authorityIndex[auth] = append(authorityIndex[auth], i)
		}
		if isHTTPURI(t.Object) {
			auth := getAuthority(t.Object)
			authorityIndex[auth] = append(authorityIndex[auth], i)
		}
	}

	if e.skosOptions.AuthorityMode == AuthorityMatchAuthority {
		for auth, indices := range authorityIndex {
			authURIs := make([]string, 0, len(indices))
			for _, idx := range indices {
				t := triples[idx]
				if isHTTPURI(t.Subject) && getAuthority(t.Subject) == auth {
					authURIs = append(authURIs, t.Subject)
				}
			}

			for i, uri1 := range authURIs {
				for _, uri2 := range authURIs[i+1:] {
					result = append(result, types.Triple{
						Subject:   uri1,
						Predicate: SKOSExactMatch,
						Object:    uri2,
						Graph:     "",
					})
					result = append(result, types.Triple{
						Subject:   uri2,
						Predicate: SKOSExactMatch,
						Object:    uri1,
						Graph:     "",
					})
				}
			}
		}
	}

	return result
}

func isHTTPURI(uri string) bool {
	return strings.HasPrefix(uri, "http://") || strings.HasPrefix(uri, "https://")
}

func getAuthority(uri string) string {
	u, err := url.Parse(uri)
	if err != nil {
		return ""
	}
	return u.Host
}
