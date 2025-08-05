package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	billing "cloud.google.com/go/billing/apiv1"
	billingpb "cloud.google.com/go/billing/apiv1/billingpb"
	"google.golang.org/api/iterator"

	"github.com/codecflow/fabric/pkg/auth"
)

func main() {
	ctx := context.Background()

	// auth via your wrapper
	gcpAuth := auth.NewGCPAuthenticator("../.testdata/gcp/key.json")
	opt, err := gcpAuth.Authenticate(ctx)
	if err != nil {
		log.Fatalf("auth failed: %v", err)
	}

	// create catalog client
	client, err := billing.NewCloudCatalogClient(ctx, opt)
	if err != nil {
		log.Fatalf("catalog client: %v", err)
	}
	defer client.Close()

	// 1) find Compute Engine service ID
	svcIt := client.ListServices(ctx, &billingpb.ListServicesRequest{PageSize: 100})
	var svcID string
	for {
		svc, err := svcIt.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Fatalf("listing services: %v", err)
		}
		if svc.DisplayName == "Compute Engine" {
			svcID = svc.Name
			break
		}
	}
	if svcID == "" {
		log.Fatal("Compute Engine service not found")
	}

	// 2) list all SKUs under that service
	skuIt := client.ListSkus(ctx, &billingpb.ListSkusRequest{
		Parent:   svcID,
		PageSize: 500,
	})

	// 3) filter in Go
	for {
		sku, err := skuIt.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Fatalf("listing SKUs: %v", err)
		}

		// only on-demand Compute instances
		cat := sku.GetCategory()
		if cat == nil ||
			cat.GetResourceFamily() != "Compute" ||
			cat.GetUsageType() != "OnDemand" ||
			!strings.Contains(strings.ToLower(sku.GetDescription()), "instance") {
			continue
		}

		// skip SKUs lacking pricing info
		pi := sku.GetPricingInfo()
		if len(pi) == 0 || pi[0].GetPricingExpression() == nil {
			continue
		}
		expr := pi[0].GetPricingExpression()
		rates := expr.GetTieredRates()
		if len(rates) == 0 {
			continue
		}

		// compute unit price
		up := rates[0].GetUnitPrice()
		price := float64(up.GetUnits()) + float64(up.GetNanos())/1e9
		unit := expr.GetUsageUnitDescription()

		// regions fallback
		regs := sku.GetGeoTaxonomy().GetRegions()
		if len(regs) == 0 {
			regs = []string{"global"}
		}
		for _, r := range regs {
			fmt.Printf("SKU=%-15s region=%-10s price=$%.4f/%s\n",
				sku.GetSkuId(), r, price, unit)
		}
	}
	fmt.Println("Done.")
}
