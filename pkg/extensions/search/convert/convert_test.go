package convert_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/99designs/gqlgen/graphql"
	godigest "github.com/opencontainers/go-digest"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	. "github.com/smartystreets/goconvey/convey"

	"zotregistry.io/zot/pkg/extensions/search/convert"
	cveinfo "zotregistry.io/zot/pkg/extensions/search/cve"
	"zotregistry.io/zot/pkg/extensions/search/gql_generated"
	"zotregistry.io/zot/pkg/log"
	"zotregistry.io/zot/pkg/meta/bolt"
	"zotregistry.io/zot/pkg/meta/repodb"
	boltdb_wrapper "zotregistry.io/zot/pkg/meta/repodb/boltdb-wrapper"
	"zotregistry.io/zot/pkg/test/mocks"
)

var ErrTestError = errors.New("TestError")

func TestConvertErrors(t *testing.T) {
	Convey("Convert Errors", t, func() {
		params := bolt.DBParameters{
			RootDir: t.TempDir(),
		}
		boltDB, err := bolt.GetBoltDriver(params)
		So(err, ShouldBeNil)

		repoDB, err := boltdb_wrapper.NewBoltDBWrapper(boltDB, log.NewLogger("debug", ""))
		So(err, ShouldBeNil)

		configBlob, err := json.Marshal(ispec.Image{})
		So(err, ShouldBeNil)

		manifestBlob, err := json.Marshal(ispec.Manifest{
			Layers: []ispec.Descriptor{
				{
					MediaType: ispec.MediaTypeImageLayerGzip,
					Size:      0,
					Digest:    godigest.NewDigestFromEncoded(godigest.SHA256, "digest"),
				},
			},
		})
		So(err, ShouldBeNil)

		repoMeta11 := repodb.ManifestMetadata{
			ManifestBlob: manifestBlob,
			ConfigBlob:   configBlob,
		}

		digest11 := godigest.FromString("abc1")
		err = repoDB.SetManifestMeta("repo1", digest11, repoMeta11)
		So(err, ShouldBeNil)
		err = repoDB.SetRepoReference("repo1", "0.1.0", digest11, ispec.MediaTypeImageManifest)
		So(err, ShouldBeNil)

		repoMetas, manifestMetaMap, _, _, err := repoDB.SearchRepos(context.Background(), "", repodb.Filter{},
			repodb.PageInput{})
		So(err, ShouldBeNil)

		ctx := graphql.WithResponseContext(context.Background(),
			graphql.DefaultErrorPresenter, graphql.DefaultRecover)

		_ = convert.RepoMeta2RepoSummary(
			ctx,
			repoMetas[0],
			manifestMetaMap,
			map[string]repodb.IndexData{},
			convert.SkipQGLField{},
			mocks.CveInfoMock{
				GetCVESummaryForImageFn: func(repo string, reference string,
				) (cveinfo.ImageCVESummary, error) {
					return cveinfo.ImageCVESummary{}, ErrTestError
				},
			},
		)

		So(graphql.GetErrors(ctx).Error(), ShouldContainSubstring, "unable to run vulnerability scan on tag")
	})

	Convey("ImageIndex2ImageSummary errors", t, func() {
		ctx := graphql.WithResponseContext(context.Background(),
			graphql.DefaultErrorPresenter, graphql.DefaultRecover)

		_, _, err := convert.ImageIndex2ImageSummary(
			ctx,
			"repo",
			"tag",
			godigest.FromString("indexDigest"),
			true,
			repodb.RepoMetadata{},
			repodb.IndexData{
				IndexBlob: []byte("bad json"),
			},
			map[string]repodb.ManifestMetadata{},
			mocks.CveInfoMock{},
		)
		So(err, ShouldNotBeNil)
	})

	Convey("ImageIndex2ImageSummary cve scanning", t, func() {
		ctx := graphql.WithResponseContext(context.Background(),
			graphql.DefaultErrorPresenter, graphql.DefaultRecover)

		_, _, err := convert.ImageIndex2ImageSummary(
			ctx,
			"repo",
			"tag",
			godigest.FromString("indexDigest"),
			false,
			repodb.RepoMetadata{},
			repodb.IndexData{
				IndexBlob: []byte("{}"),
			},
			map[string]repodb.ManifestMetadata{},
			mocks.CveInfoMock{
				GetCVESummaryForImageFn: func(repo, reference string,
				) (cveinfo.ImageCVESummary, error) {
					return cveinfo.ImageCVESummary{}, ErrTestError
				},
			},
		)
		So(err, ShouldBeNil)
	})

	Convey("ImageManifest2ImageSummary", t, func() {
		ctx := graphql.WithResponseContext(context.Background(),
			graphql.DefaultErrorPresenter, graphql.DefaultRecover)
		configBlob, err := json.Marshal(ispec.Image{
			Platform: ispec.Platform{
				OS:           "os",
				Architecture: "arch",
				Variant:      "var",
			},
		})
		So(err, ShouldBeNil)

		_, _, err = convert.ImageManifest2ImageSummary(
			ctx,
			"repo",
			"tag",
			godigest.FromString("manifestDigest"),
			false,
			repodb.RepoMetadata{},
			repodb.ManifestMetadata{
				ManifestBlob: []byte("{}"),
				ConfigBlob:   configBlob,
			},
			mocks.CveInfoMock{
				GetCVESummaryForImageFn: func(repo, reference string,
				) (cveinfo.ImageCVESummary, error) {
					return cveinfo.ImageCVESummary{}, ErrTestError
				},
			},
		)
		So(err, ShouldBeNil)
	})

	Convey("ImageManifest2ManifestSummary", t, func() {
		ctx := graphql.WithResponseContext(context.Background(),
			graphql.DefaultErrorPresenter, graphql.DefaultRecover)

		// with bad config json, error while unmarshaling
		_, _, err := convert.ImageManifest2ManifestSummary(
			ctx,
			"repo",
			"tag",
			ispec.Descriptor{
				Digest:    "dig",
				MediaType: ispec.MediaTypeImageManifest,
			},
			false,
			repodb.RepoMetadata{
				Tags:       map[string]repodb.Descriptor{},
				Statistics: map[string]repodb.DescriptorStatistics{},
				Signatures: map[string]repodb.ManifestSignatures{},
				Referrers:  map[string][]repodb.ReferrerInfo{},
			},
			repodb.ManifestMetadata{
				ManifestBlob: []byte("{}"),
				ConfigBlob:   []byte("bad json"),
			},
			nil,
			mocks.CveInfoMock{
				GetCVESummaryForImageFn: func(repo, reference string,
				) (cveinfo.ImageCVESummary, error) {
					return cveinfo.ImageCVESummary{}, ErrTestError
				},
			},
		)
		So(err, ShouldNotBeNil)

		// CVE scan using platform
		configBlob, err := json.Marshal(ispec.Image{
			Platform: ispec.Platform{
				OS:           "os",
				Architecture: "arch",
				Variant:      "var",
			},
		})
		So(err, ShouldBeNil)

		_, _, err = convert.ImageManifest2ManifestSummary(
			ctx,
			"repo",
			"tag",
			ispec.Descriptor{
				Digest:    "dig",
				MediaType: ispec.MediaTypeImageManifest,
			},
			false,
			repodb.RepoMetadata{
				Tags:       map[string]repodb.Descriptor{},
				Statistics: map[string]repodb.DescriptorStatistics{},
				Signatures: map[string]repodb.ManifestSignatures{"dig": {"cosine": []repodb.SignatureInfo{{}}}},
				Referrers:  map[string][]repodb.ReferrerInfo{},
			},
			repodb.ManifestMetadata{
				ManifestBlob: []byte("{}"),
				ConfigBlob:   configBlob,
			},
			nil,
			mocks.CveInfoMock{
				GetCVESummaryForImageFn: func(repo, reference string,
				) (cveinfo.ImageCVESummary, error) {
					return cveinfo.ImageCVESummary{}, ErrTestError
				},
			},
		)
		So(err, ShouldBeNil)
	})

	Convey("RepoMeta2ExpandedRepoInfo", t, func() {
		ctx := graphql.WithResponseContext(context.Background(),
			graphql.DefaultErrorPresenter, graphql.DefaultRecover)

		// with bad config json, error while unmarshaling
		_, imageSummaries := convert.RepoMeta2ExpandedRepoInfo(
			ctx,
			repodb.RepoMetadata{
				Tags: map[string]repodb.Descriptor{
					"tag1": {Digest: "dig", MediaType: ispec.MediaTypeImageManifest},
				},
			},
			map[string]repodb.ManifestMetadata{
				"dig": {
					ManifestBlob: []byte("{}"),
					ConfigBlob:   []byte("bad json"),
				},
			},
			map[string]repodb.IndexData{},
			convert.SkipQGLField{
				Vulnerabilities: false,
			},
			mocks.CveInfoMock{
				GetCVESummaryForImageFn: func(repo, reference string,
				) (cveinfo.ImageCVESummary, error) {
					return cveinfo.ImageCVESummary{}, ErrTestError
				},
			}, log.NewLogger("debug", ""),
		)
		So(len(imageSummaries), ShouldEqual, 0)

		// cveInfo present no error
		_, imageSummaries = convert.RepoMeta2ExpandedRepoInfo(
			ctx,
			repodb.RepoMetadata{
				Tags: map[string]repodb.Descriptor{
					"tag1": {Digest: "dig", MediaType: ispec.MediaTypeImageManifest},
				},
			},
			map[string]repodb.ManifestMetadata{
				"dig": {
					ManifestBlob: []byte("{}"),
					ConfigBlob:   []byte("{}"),
				},
			},
			map[string]repodb.IndexData{},
			convert.SkipQGLField{
				Vulnerabilities: false,
			},
			mocks.CveInfoMock{
				GetCVESummaryForImageFn: func(repo, reference string,
				) (cveinfo.ImageCVESummary, error) {
					return cveinfo.ImageCVESummary{}, ErrTestError
				},
			}, log.NewLogger("debug", ""),
		)
		So(len(imageSummaries), ShouldEqual, 1)
	})
}

func TestUpdateLastUpdatedTimestam(t *testing.T) {
	Convey("Image summary is the first image checked for the repo", t, func() {
		before := time.Time{}
		after := time.Date(2023, time.April, 1, 11, 0, 0, 0, time.UTC)
		img := convert.UpdateLastUpdatedTimestamp(
			&before,
			&gql_generated.ImageSummary{LastUpdated: &before},
			&gql_generated.ImageSummary{LastUpdated: &after},
		)

		So(*img.LastUpdated, ShouldResemble, after)
	})

	Convey("Image summary is updated after the current latest image", t, func() {
		before := time.Date(2022, time.April, 1, 11, 0, 0, 0, time.UTC)
		after := time.Date(2023, time.April, 1, 11, 0, 0, 0, time.UTC)
		img := convert.UpdateLastUpdatedTimestamp(
			&before,
			&gql_generated.ImageSummary{LastUpdated: &before},
			&gql_generated.ImageSummary{LastUpdated: &after},
		)

		So(*img.LastUpdated, ShouldResemble, after)
	})

	Convey("Image summary is updated before the current latest image", t, func() {
		before := time.Date(2022, time.April, 1, 11, 0, 0, 0, time.UTC)
		after := time.Date(2023, time.April, 1, 11, 0, 0, 0, time.UTC)
		img := convert.UpdateLastUpdatedTimestamp(
			&after,
			&gql_generated.ImageSummary{LastUpdated: &after},
			&gql_generated.ImageSummary{LastUpdated: &before},
		)

		So(*img.LastUpdated, ShouldResemble, after)
	})
}
