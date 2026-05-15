package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/XDWow/DouyinMall/backend/pkg/envx"
	"github.com/spf13/pflag"
)

const (
	modePrepare = "prepare"
	modeStore   = "store"
	modeFull    = "full"
)

type cliFlags struct {
	mode         string
	configPath   string
	sourceURL    string
	artifactPath string
	knowledgeID  string
	title        string
	category     string
	selector     string
	replace      bool
}

func main() {
	if err := envx.Load(); err != nil {
		log.Fatalf("load .env failed: %v", err)
	}

	flags := parseFlags()
	cfg := initConfig(flags.configPath, flags.mode)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	switch flags.mode {
	case modePrepare:
		runPrepare(ctx, cfg, flags)
	case modeStore:
		runStore(ctx, cfg, flags)
	case modeFull:
		runFull(ctx, cfg, flags)
	default:
		log.Fatalf("unsupported mode: %s", flags.mode)
	}
}

func parseFlags() cliFlags {
	var flags cliFlags

	pflag.StringVar(&flags.mode, "mode", modeFull, "run mode: prepare | store | full")
	pflag.StringVar(&flags.configPath, "config", "cmd/knowledge-init/host.yaml", "knowledge init config file path")
	pflag.StringVar(&flags.sourceURL, "url", "", "source URL to ingest")
	pflag.StringVar(&flags.artifactPath, "artifact", "", "artifact file path for prepare output or store input")
	pflag.StringVar(&flags.knowledgeID, "knowledge-id", "", "stable knowledge id, defaults to a value derived from the source URL")
	pflag.StringVar(&flags.title, "title", "", "knowledge title override")
	pflag.StringVar(&flags.category, "category", "", "knowledge category override")
	pflag.StringVar(&flags.selector, "selector", "", "optional CSS selector used by the generic HTML loader")
	pflag.BoolVar(&flags.replace, "replace", true, "delete existing chunks under the same knowledge id before storing new chunks")
	pflag.Parse()

	flags.mode = strings.ToLower(strings.TrimSpace(flags.mode))
	switch flags.mode {
	case modePrepare, modeFull:
		if strings.TrimSpace(flags.sourceURL) == "" && len(pflag.Args()) > 0 {
			flags.sourceURL = pflag.Arg(0)
		}
		if strings.TrimSpace(flags.sourceURL) == "" {
			log.Fatal("source URL is required, use --url")
		}
	case modeStore:
		if strings.TrimSpace(flags.artifactPath) == "" && len(pflag.Args()) > 0 {
			flags.artifactPath = pflag.Arg(0)
		}
		if strings.TrimSpace(flags.artifactPath) == "" {
			log.Fatal("artifact path is required in store mode, use --artifact")
		}
	default:
		log.Fatalf("invalid mode: %s", flags.mode)
	}

	return flags
}

func runPrepare(ctx context.Context, cfg Config, flags cliFlags) {
	service, cleanup, err := newPrepareService(ctx, cfg, flags.selector)
	if err != nil {
		log.Fatalf("init prepare service failed: %v", err)
	}
	defer cleanup()

	artifact, err := service.PrepareURL(ctx, PrepareRequest{
		URL:         flags.sourceURL,
		KnowledgeID: flags.knowledgeID,
		Title:       flags.title,
		Category:    flags.category,
	})
	if err != nil {
		log.Fatalf("prepare knowledge artifact failed: %v", err)
	}

	artifactPath := strings.TrimSpace(flags.artifactPath)
	if artifactPath == "" {
		artifactPath = defaultArtifactPath("tmp", artifact.KnowledgeID)
	}
	if err := writeArtifact(artifactPath, artifact); err != nil {
		log.Fatalf("write artifact failed: %v", err)
	}

	fmt.Printf(
		"mode=%s\nartifact=%s\nknowledge_id=%s\ntitle=%s\ncategory=%s\nraw_documents=%d\nchunks=%d\nsource=%s\n",
		modePrepare,
		artifactPath,
		artifact.KnowledgeID,
		artifact.Title,
		artifact.Category,
		len(artifact.Documents),
		len(artifact.Chunks),
		artifact.SourceURL,
	)
}

func runStore(ctx context.Context, cfg Config, flags cliFlags) {
	artifact, err := readArtifact(flags.artifactPath)
	if err != nil {
		log.Fatalf("read artifact failed: %v", err)
	}

	service, cleanup, err := newStoreService(ctx, cfg)
	if err != nil {
		log.Fatalf("init store service failed: %v", err)
	}
	defer cleanup()

	result, err := service.StoreArtifact(ctx, StoreRequest{
		Artifact: artifact,
		Replace:  flags.replace,
	})
	if err != nil {
		log.Fatalf("store knowledge artifact failed: %v", err)
	}

	printStoreResult(modeStore, flags.artifactPath, result)
}

func runFull(ctx context.Context, cfg Config, flags cliFlags) {
	prepareService, prepareCleanup, err := newPrepareService(ctx, cfg, flags.selector)
	if err != nil {
		log.Fatalf("init prepare service failed: %v", err)
	}
	defer prepareCleanup()

	artifact, err := prepareService.PrepareURL(ctx, PrepareRequest{
		URL:         flags.sourceURL,
		KnowledgeID: flags.knowledgeID,
		Title:       flags.title,
		Category:    flags.category,
	})
	if err != nil {
		log.Fatalf("prepare knowledge artifact failed: %v", err)
	}

	if artifactPath := strings.TrimSpace(flags.artifactPath); artifactPath != "" {
		if err := writeArtifact(artifactPath, artifact); err != nil {
			log.Fatalf("write artifact failed: %v", err)
		}
	}

	storeService, storeCleanup, err := newStoreService(ctx, cfg)
	if err != nil {
		log.Fatalf("init store service failed: %v", err)
	}
	defer storeCleanup()

	result, err := storeService.StoreArtifact(ctx, StoreRequest{
		Artifact: artifact,
		Replace:  flags.replace,
	})
	if err != nil {
		log.Fatalf("store knowledge artifact failed: %v", err)
	}

	printStoreResult(modeFull, strings.TrimSpace(flags.artifactPath), result)
}

func printStoreResult(mode string, artifactPath string, result *IngestResult) {
	fmt.Printf(
		"mode=%s\nartifact=%s\nknowledge_id=%s\ntitle=%s\ncategory=%s\nraw_documents=%d\nchunks=%d\nsource=%s\n",
		mode,
		artifactPath,
		result.KnowledgeID,
		result.Title,
		result.Category,
		result.RawDocumentCount,
		result.ChunkCount,
		result.SourceURL,
	)
}
