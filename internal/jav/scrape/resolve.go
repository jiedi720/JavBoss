package scrape

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ResolveJavByCodes queries the default client using the automatic provider strategy.
func ResolveJavByCodes(ctx context.Context, possibleCodes, uncensoredPossibleCodes []string) (*JavInfo, error) {
	return defaultMetadataClient.ResolveJavByCodes(ctx, possibleCodes, uncensoredPossibleCodes)
}

// ResolveJavByCodes resolves each candidate through the multi-source aggregator,
// then tries uncensoredPossibleCodes through Avsox. The aggregator queries all
// configured sources in parallel and merges their fields by priority. It returns
// the first successful code. Empty results and ErrNotFound advance to the next
// candidate. Other lookup errors are collected and returned only if no lookup
// succeeds; ErrNotFound is returned when all lookups miss.
// Cancellation or expiration of ctx stops resolution immediately.
func (c *MetadataClient) ResolveJavByCodes(ctx context.Context, possibleCodes, uncensoredPossibleCodes []string) (*JavInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var failures []error
	for _, code := range possibleCodes {
		info, err := c.resolveAggregated(ctx, code)
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		if err != nil {
			if !errors.Is(err, ErrNotFound) {
				failures = append(failures, fmt.Errorf("resolve jav code=%s: %w", code, err))
			}
			continue
		}
		if info != nil {
			return info, nil
		}
	}
	for _, code := range uncensoredPossibleCodes {
		info, err := c.LookupJavByCode(ctx, code, ProviderAvsox)
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		if err != nil {
			if !errors.Is(err, ErrNotFound) {
				failures = append(failures, fmt.Errorf("resolve jav avsox code=%s: %w", code, err))
			}
			continue
		}
		if info != nil {
			return info, nil
		}
	}
	if len(failures) > 0 {
		return nil, errors.Join(failures...)
	}
	return nil, ErrNotFound
}

func providersForCode(code string) []Provider {
	code = strings.ToUpper(strings.TrimSpace(code))
	switch {
	case strings.HasPrefix(code, "FC2-PPV-"):
		return []Provider{ProviderJavDBAPI}
	case strings.HasPrefix(code, "GANA-"):
		return []Provider{ProviderJavMenu, ProviderJavBus}
	case strings.HasPrefix(code, "STARS-"):
		return []Provider{ProviderJavBus, ProviderAvmoo}
	case strings.HasPrefix(code, "AP-"):
		return []Provider{ProviderAvmoo}
	default:
		return []Provider{ProviderJavBus, ProviderJavDB}
	}
}
