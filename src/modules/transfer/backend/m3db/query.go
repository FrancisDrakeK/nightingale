package m3db

import (
	"strings"
	"time"

	"github.com/didi/nightingale/src/common/dataobj"
	"github.com/m3db/m3/src/dbnode/storage/index"
	"github.com/m3db/m3/src/m3ninx/idx"
)

// QueryData
func queryDataOptions(inputs []dataobj.QueryData) (index.Query, index.QueryOptions) {
	q := []idx.Query{}

	for _, input := range inputs {
		q1 := endpointsQuery(input.Nids, input.Endpoints)
		q2 := counterQuery(input.Counters)
		q = append(q, idx.NewConjunctionQuery(q1, q2))
	}

	return index.Query{idx.NewDisjunctionQuery(q...)},
		index.QueryOptions{
			StartInclusive: time.Unix(inputs[0].Start, 0),
			EndExclusive:   time.Unix(inputs[0].End, 0),
			DocsLimit:      DOCS_LIMIT,
			SeriesLimit:    SERIES_LIMIT,
		}
}

// QueryDataForUI
// metric && (endpoints[0] || endporint[1] ...) && (tags[0] || tags[1] ...)
func queryDataUIOptions(input dataobj.QueryDataForUI) (index.Query, index.QueryOptions) {
	q1 := idx.NewTermQuery([]byte(METRIC_NAME), []byte(input.Metric))
	q2 := endpointsQuery(input.Nids, input.Endpoints)
	q3 := metricTagsQuery(input.Tags)

	return index.Query{idx.NewConjunctionQuery(q1, q2, q3)},
		index.QueryOptions{
			StartInclusive: time.Unix(input.Start, 0),
			EndExclusive:   time.Unix(input.End, 0),
			SeriesLimit:    SERIES_LIMIT,
			DocsLimit:      DOCS_LIMIT,
		}
}

func metricsQuery(metrics []string) idx.Query {
	q := []idx.Query{}
	for _, v := range metrics {
		q = append(q, idx.NewTermQuery([]byte(METRIC_NAME), []byte(v)))
	}
	return idx.NewDisjunctionQuery(q...)
}

func metricQuery(metric string) idx.Query {
	return idx.NewTermQuery([]byte(METRIC_NAME), []byte(metric))
}

func endpointsQuery(nids, endpoints []string) idx.Query {
	if len(nids) > 0 {
		q := []idx.Query{}
		for _, v := range nids {
			q = append(q, idx.NewTermQuery([]byte(NID_NAME), []byte(v)))
		}
		return idx.NewDisjunctionQuery(q...)
	}

	if len(endpoints) > 0 {
		q := []idx.Query{}
		for _, v := range endpoints {
			q = append(q, idx.NewTermQuery([]byte(ENDPOINT_NAME), []byte(v)))
		}
		return idx.NewDisjunctionQuery(q...)
	}

	return idx.NewAllQuery()
}

func counterQuery(counters []string) idx.Query {
	q := []idx.Query{}

	for _, v := range counters {
		items := strings.SplitN(v, "/", 2)

		if len(items) != 2 {
			continue
		}

		tagMap := dataobj.DictedTagstring(items[1])

		q2 := []idx.Query{}
		q2 = append(q2, idx.NewTermQuery([]byte(METRIC_NAME), []byte(items[0])))

		for k, v := range tagMap {
			q2 = append(q2, idx.NewTermQuery([]byte(k), []byte(v)))
		}
		q = append(q, idx.NewConjunctionQuery(q2...))
	}

	if len(q) > 0 {
		return idx.NewDisjunctionQuery(q...)
	}

	return idx.NewAllQuery()
}

// (tags[0] || tags[2] || ...)
func metricTagsQuery(tags []string) idx.Query {
	if len(tags) == 0 {
		return idx.NewAllQuery()
	}

	q := []idx.Query{}
	for _, v := range tags {
		q1 := []idx.Query{}
		tagMap := dataobj.DictedTagstring(v)

		for k, v := range tagMap {
			q1 = append(q1, idx.NewTermQuery([]byte(k), []byte(v)))
		}
		q = append(q, idx.NewConjunctionQuery(q1...))
	}

	return idx.NewDisjunctionQuery(q...)
}

// QueryMetrics
// (endpoint[0] || endpoint[1] ... )
func queryMetricsOptions(input dataobj.EndpointsRecv) (index.Query, index.AggregationOptions) {
	nameByte := []byte(METRIC_NAME)
	return index.Query{idx.NewConjunctionQuery(
			endpointsQuery(nil, input.Endpoints),
			idx.NewFieldQuery(nameByte),
		)},
		index.AggregationOptions{
			QueryOptions: index.QueryOptions{
				StartInclusive: time.Time{},
				EndExclusive:   time.Now(),
				SeriesLimit:    SERIES_LIMIT,
				DocsLimit:      DOCS_LIMIT,
			},
			FieldFilter: [][]byte{nameByte},
			Type:        index.AggregateTagNamesAndValues,
		}
}

// QueryTagPairs
// (endpoint[0] || endpoint[1]...) && (metrics[0] || metrics[1] ... )
func queryTagPairsOptions(input dataobj.EndpointMetricRecv) (index.Query, index.AggregationOptions) {
	q1 := endpointsQuery(nil, input.Endpoints)
	q2 := metricsQuery(input.Metrics)

	return index.Query{idx.NewConjunctionQuery(q1, q2)},
		index.AggregationOptions{
			QueryOptions: index.QueryOptions{
				StartInclusive: time.Time{},
				EndExclusive:   time.Now(),
				SeriesLimit:    SERIES_LIMIT,
				DocsLimit:      DOCS_LIMIT,
			},
			FieldFilter: index.AggregateFieldFilter(nil),
			Type:        index.AggregateTagNamesAndValues,
		}
}

// QueryIndexByClude: || (&& (|| endpoints...) (metric) (|| include...) (&& exclude..))
func queryIndexByCludeOptions(input dataobj.CludeRecv) (index.Query, index.QueryOptions) {
	query := index.Query{}
	q := []idx.Query{}

	if len(input.Endpoints) > 0 {
		q = append(q, endpointsQuery(nil, input.Endpoints))
	}
	if input.Metric != "" {
		q = append(q, metricQuery(input.Metric))
	}
	if len(input.Include) > 0 {
		q = append(q, includeTagsQuery(input.Include))
	}
	if len(input.Exclude) > 0 {
		q = append(q, excludeTagsQuery(input.Exclude))
	}

	if len(q) == 0 {
		query = index.Query{idx.NewAllQuery()}
	} else {
		query = index.Query{idx.NewDisjunctionQuery(q...)}
	}

	return query, index.QueryOptions{
		StartInclusive: time.Time{},
		EndExclusive:   time.Now(),
		SeriesLimit:    SERIES_LIMIT,
		DocsLimit:      DOCS_LIMIT,
	}

}

// QueryIndexByFullTags: && (|| endpoints) (metric) (&& tagkv)
func queryIndexByFullTagsOptions(input dataobj.IndexByFullTagsRecv) (index.Query, index.QueryOptions) {
	query := index.Query{}
	q := []idx.Query{}

	if len(input.Endpoints) > 0 {
		q = append(q, endpointsQuery(nil, input.Endpoints))
	}
	if input.Metric != "" {
		q = append(q, metricQuery(input.Metric))
	}
	if len(input.Tagkv) > 0 {
		q = append(q, includeTagsQuery2(input.Tagkv))
	}

	if len(q) == 0 {
		query = index.Query{idx.NewAllQuery()}
	} else {
		query = index.Query{idx.NewConjunctionQuery(q...)}
	}

	return query, index.QueryOptions{
		StartInclusive: time.Time{},
		EndExclusive:   time.Now(),
		SeriesLimit:    SERIES_LIMIT,
		DocsLimit:      DOCS_LIMIT,
	}
}

// && ((|| values...))...
func includeTagsQuery(in []*dataobj.TagPair) idx.Query {
	q := []idx.Query{}
	for _, kvs := range in {
		q1 := []idx.Query{}
		for _, v := range kvs.Values {
			q1 = append(q1, idx.NewTermQuery([]byte(kvs.Key), []byte(v)))
		}
		if len(q1) > 0 {
			q = append(q, idx.NewDisjunctionQuery(q1...))
		}
	}

	if len(q) == 0 {
		return idx.NewAllQuery()
	}

	return idx.NewConjunctionQuery(q...)
}

func includeTagsQuery2(in []dataobj.TagPair) idx.Query {
	q := []idx.Query{}
	for _, kvs := range in {
		q1 := []idx.Query{}
		for _, v := range kvs.Values {
			q1 = append(q1, idx.NewTermQuery([]byte(kvs.Key), []byte(v)))
		}
		if len(q1) > 0 {
			q = append(q, idx.NewDisjunctionQuery(q1...))
		}
	}

	if len(q) == 0 {
		return idx.NewAllQuery()
	}

	return idx.NewConjunctionQuery(q...)
}

// && (&& !values...)
func excludeTagsQuery(in []*dataobj.TagPair) idx.Query {
	q := []idx.Query{}
	for _, kvs := range in {
		q1 := []idx.Query{}
		for _, v := range kvs.Values {
			q1 = append(q1, idx.NewNegationQuery(idx.NewTermQuery([]byte(kvs.Key), []byte(v))))
		}
		if len(q1) > 0 {
			q = append(q, idx.NewConjunctionQuery(q1...))
		}
	}

	if len(q) == 0 {
		return idx.NewAllQuery()
	}

	return idx.NewConjunctionQuery(q...)
}
