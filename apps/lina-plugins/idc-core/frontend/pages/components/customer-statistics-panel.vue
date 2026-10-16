<script setup lang="ts">
import type { CustomerOperationsStatistics } from "../customer-client";
import { computed } from "vue";
import { Button } from "ant-design-vue";
import { $t } from "#/locales";
import { minorToMajorAmount } from "../catalog-money";
const props = defineProps<{ statistics: CustomerOperationsStatistics }>();
const emit = defineEmits<{ viewProducts: [] }>();
const text = (key: string) => $t(`plugin.idc-core.customer.workspace.${key}`);
const categories = computed(() => {
  const totals = new Map<string, number>();
  for (const item of props.statistics.productCategories || []) {
    const label =
      item.pluginId === "yunsu-ssl"
        ? "ssl"
        : item.productType === "bare_metal"
          ? "dedicatedServer"
          : item.productType === "cloud_server"
            ? "cloudServer"
            : "otherProduct";
    totals.set(label, (totals.get(label) || 0) + item.total);
  }
  return [...totals].map(([key, total]) => ({ key, total }));
});
</script>

<template>
  <section class="statistics" data-testid="customer-statistics">
    <div class="ticket-stats">
      <h3>{{ text("tickets") }}</h3>
      <div
        class="stat-line"
        v-for="item in [
          {
            key: 'ticketTotal',
            value: statistics.tickets.total,
            color: 'blue',
          },
          {
            key: 'ticketProcessing',
            value: statistics.tickets.processing,
            color: 'orange',
          },
          {
            key: 'ticketReplied',
            value: statistics.tickets.replied,
            color: 'green',
          },
          {
            key: 'ticketClosed',
            value: statistics.tickets.closed,
            color: 'grey',
          },
        ]"
        :key="item.key"
      >
        <i :class="item.color"></i>
        <div>
          <span>{{ text(item.key) }}</span
          ><strong>{{ item.value }}</strong>
        </div>
      </div>
    </div>
    <div class="finance-stats">
      <h3>{{ text("financialOverview") }}</h3>
      <div
        v-for="currency in statistics.currencies"
        :key="currency.currencyCode"
        class="money-grid"
      >
        <article
          v-for="item in [
            { key: 'totalPaid', amount: currency.paidMinor },
            { key: 'totalRefunded', amount: currency.refundedMinor },
            { key: 'unpaid', amount: currency.unpaidMinor },
          ]"
          :key="item.key"
        >
          <span>{{ text(item.key) }}</span
          ><strong
            >{{ currency.currencyCode }}
            {{ minorToMajorAmount(item.amount, currency.currencyCode) }}</strong
          ><small v-if="item.key === 'unpaid'"
            >{{ currency.unpaidCount }} {{ text("invoices") }}</small
          >
        </article>
      </div>
      <p v-if="!statistics.currencies.length" class="empty-finance">
        {{ text("noFinancialRecords") }}
      </p>
    </div>
    <div class="product-section" data-testid="customer-product-statistics">
      <div class="section-heading">
        <h3>{{ text("productsAndServices") }}</h3>
        <Button size="small" @click="emit('viewProducts')">{{
          text("viewProducts")
        }}</Button>
      </div>
      <div class="product-stats">
        <article>
          <span>{{ text("products") }}</span
          ><strong>{{ statistics.products }}</strong>
        </article>
        <article>
          <span>{{ text("expiring") }}</span
          ><strong class="warning">{{ statistics.expiring }}</strong
          ><small>{{ text("next30Days") }}</small>
        </article>
        <article>
          <span>{{ text("expired") }}</span
          ><strong class="danger">{{ statistics.expired }}</strong>
        </article>
      </div>
      <div
        v-if="categories.length"
        class="category-grid"
        data-testid="customer-product-categories"
      >
        <article
          v-for="item in categories"
          :key="item.key"
          :data-testid="`customer-category-${item.key}`"
        >
          <strong>{{ item.total }}</strong
          ><span>{{ text(item.key) }}</span>
        </article>
      </div>
      <p v-else class="empty-finance">{{ text("noProducts") }}</p>
    </div>
  </section>
</template>

<style scoped>
.statistics {
  display: grid;
  grid-template-columns: 180px minmax(0, 1fr);
  gap: 24px;
  padding: 20px;
  border: 1px solid hsl(var(--border));
  border-radius: 8px;
  background: hsl(var(--background));
}
h3 {
  font-size: 15px;
  font-weight: 600;
  margin: 0 0 20px;
}
.stat-line {
  display: flex;
  gap: 12px;
  align-items: center;
  margin: 0 0 16px;
}
.stat-line i {
  height: 34px;
  width: 5px;
  border-radius: 4px;
  background: hsl(var(--primary));
}
.stat-line .orange {
  background: #faad14;
}
.stat-line .green {
  background: #52c41a;
}
.stat-line .grey {
  background: #bfbfbf;
}
.stat-line span,
article span {
  display: block;
  color: hsl(var(--muted-foreground));
  font-size: 13px;
}
.stat-line strong {
  font-size: 21px;
  font-weight: 500;
}
.section-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 20px;
}
.section-heading h3 {
  margin: 0;
}
.category-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
  gap: 12px;
  margin-top: 20px;
}
.category-grid article {
  display: flex;
  align-items: center;
  gap: 14px;
  border: 1px solid hsl(var(--border));
  border-radius: 6px;
  padding: 16px;
}
.category-grid strong {
  color: hsl(var(--primary));
  margin: 0;
  font-size: 24px;
}
.product-section {
  grid-column: 1 / -1;
  padding-top: 20px;
  border-top: 1px solid hsl(var(--border));
}
.finance-stats {
  min-width: 0;
}
.money-grid,
.product-stats {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 20px;
}
.money-grid {
  padding-bottom: 20px;
  margin-bottom: 18px;
  border-bottom: 1px solid hsl(var(--border));
}
article strong {
  display: block;
  margin-top: 10px;
  font-size: 22px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  overflow-wrap: anywhere;
}
article small {
  display: block;
  margin-top: 6px;
  color: hsl(var(--muted-foreground));
}
.warning {
  color: #d48806;
}
.danger {
  color: #cf1322;
}
.empty-finance {
  color: hsl(var(--muted-foreground));
  padding: 8px 0 20px;
}
@media (max-width: 760px) {
  .statistics {
    grid-template-columns: 1fr;
  }
  .ticket-stats {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .ticket-stats h3 {
    grid-column: 1/-1;
  }
  .money-grid,
  .product-stats {
    gap: 12px;
  }
  article strong {
    font-size: 18px;
  }
}
</style>
