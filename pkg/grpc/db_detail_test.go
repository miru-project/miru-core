package grpc

import (
	"context"
	"testing"

	"github.com/miru-project/miru-core/ent/enttest"
	"github.com/miru-project/miru-core/ext"
	pb "github.com/miru-project/miru-core/proto/generate/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

// TestDetailTagsCacheRoundTrip proves the detail cache carries tags over the
// DbService API: an extension detail saved with tags comes back with them on
// GetDetail, and a later save that omits tags (an older frontend or a partial
// refresh) must not wipe the cached list.
func TestDetailTagsCacheRoundTrip(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:grpcdetailtags?mode=memory&cache=shared&_fk=1&_pragma=foreign_keys(1)")
	defer client.Close()
	ext.SetEntClientForTest(client)
	defer ext.SetEntClientForTest(nil)

	ctx := context.Background()
	s := &MiruCoreServer{}
	pkg := "com.example.tags"
	url := "https://example.com/detail/1"

	title := "Tagged Detail"
	saved, err := s.UpsertDetail(ctx, &pb.UpsertDetailRequest{
		Package:   pkg,
		DetailUrl: url,
		Title:     &title,
		Tags:      []string{"Action", "Comedy"},
	})
	require.NoError(t, err)
	require.NotNil(t, saved.GetDetail())
	assert.Equal(t, []string{"Action", "Comedy"}, saved.GetDetail().GetTags(),
		"UpsertDetail must persist the tags it was given")

	got, err := s.GetDetail(ctx, &pb.GetDetailRequest{Package: pkg, DetailUrl: url})
	require.NoError(t, err)
	assert.Equal(t, []string{"Action", "Comedy"}, got.GetDetail().GetTags(),
		"GetDetail must return the cached tags")

	// A refresh that omits tags keeps the cached list.
	newTitle := "Refreshed Detail"
	_, err = s.UpsertDetail(ctx, &pb.UpsertDetailRequest{
		Package:   pkg,
		DetailUrl: url,
		Title:     &newTitle,
	})
	require.NoError(t, err)
	got, err = s.GetDetail(ctx, &pb.GetDetailRequest{Package: pkg, DetailUrl: url})
	require.NoError(t, err)
	assert.Equal(t, []string{"Action", "Comedy"}, got.GetDetail().GetTags(),
		"an upsert that omits tags must not wipe the cached list")
	assert.Equal(t, newTitle, got.GetDetail().GetTitle())

	// A refresh that carries new tags replaces them.
	refreshTitle := "Refreshed With Tags"
	_, err = s.UpsertDetail(ctx, &pb.UpsertDetailRequest{
		Package:   pkg,
		DetailUrl: url,
		Title:     &refreshTitle,
		Tags:      []string{"Drama"},
	})
	require.NoError(t, err)
	got, err = s.GetDetail(ctx, &pb.GetDetailRequest{Package: pkg, DetailUrl: url})
	require.NoError(t, err)
	assert.Equal(t, []string{"Drama"}, got.GetDetail().GetTags())
}
