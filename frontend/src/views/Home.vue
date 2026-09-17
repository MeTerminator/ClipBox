<template>
  <section
    class="pickup-page animate-in fade-in slide-in-from-bottom-4 duration-500"
  >
    <div class="pickup-heading">
      <h1>{{ $t("home.pickupTitle") }}</h1>
      <p>{{ $t("home.pickupSubtitle") }}</p>
    </div>

    <form @submit.prevent="pickup" class="pickup-form">
      <div
        class="pickup-code-grid"
        role="group"
        :aria-label="$t('home.pickupCode')"
      >
        <input
          v-for="(_, index) in digits"
          :key="index"
          :ref="(element) => setInputRef(element, index)"
          v-model="digits[index]"
          inputmode="numeric"
          autocomplete="one-time-code"
          maxlength="1"
          pattern="[0-9]"
          :aria-label="$t('home.digitLabel', { number: index + 1 })"
          @input="handleInput(index, $event)"
          @keydown="handleKeydown(index, $event)"
          @paste="handlePaste"
        />
      </div>

      <Button type="submit" class="pickup-submit" :disabled="loading">
        <CloudDownloadIcon class="h-5 w-5" />
        {{ loading ? $t("home.loading") : $t("home.pickupBtn") }}
      </Button>

      <div class="pickup-actions">
        <router-link
          :to="{ name: 'create', query: { tab: 'file' } }"
          class="pickup-secondary-action"
        >
          <UploadCloudIcon class="h-5 w-5" />
          {{ $t("home.sendFile") }}
        </router-link>
        <button
          type="button"
          class="pickup-secondary-action"
          @click="historyOpen = true"
        >
          <HistoryIcon class="h-5 w-5" />
          {{ $t("home.pickupHistory") }}
        </button>
        <router-link
          :to="{ name: 'rooms' }"
          class="pickup-secondary-action pickup-room-action"
        >
          <MessagesSquareIcon class="h-5 w-5" />
          {{ $t("rooms.title") }}
        </router-link>
      </div>
    </form>
  </section>

  <PickupResultModal
    :open="resultOpen"
    :result="pickupResult"
    @close="resultOpen = false"
    @copied="toast.success(t('clip.copied'))"
  />
  <PickupHistoryModal
    :open="historyOpen"
    :records="pickupHistory"
    @close="historyOpen = false"
    @select="openHistoryRecord"
    @clear="clearPickupHistory"
  />
</template>

<script setup>
import { nextTick, ref } from "vue";
import { useI18n } from "vue-i18n";
import { toast } from "vue-sonner";
import {
  CloudDownloadIcon,
  HistoryIcon,
  MessagesSquareIcon,
  UploadCloudIcon,
} from "lucide-vue-next";
import { Button } from "@/components/ui/button";
import PickupHistoryModal from "@/components/PickupHistoryModal.vue";
import PickupResultModal from "@/components/PickupResultModal.vue";

const { t } = useI18n();
const digits = ref(["", "", "", "", ""]);
const inputRefs = ref([]);
const loading = ref(false);
const pickupResult = ref(null);
const resultOpen = ref(false);
const historyOpen = ref(false);
const pickupHistory = ref(
  JSON.parse(localStorage.getItem("pickupHistory") || "[]"),
);

const setInputRef = (element, index) => {
  if (element) inputRefs.value[index] = element;
};

const handleInput = (index, event) => {
  const value = event.target.value.replace(/\D/g, "").slice(-1);
  digits.value[index] = value;
  if (value && index < digits.value.length - 1) {
    nextTick(() => inputRefs.value[index + 1]?.focus());
  } else if (value && index === digits.value.length - 1) {
    nextTick(pickup);
  }
};

const handleKeydown = (index, event) => {
  if (event.key === "Backspace" && !digits.value[index] && index > 0) {
    digits.value[index - 1] = "";
    nextTick(() => inputRefs.value[index - 1]?.focus());
  }
};

const handlePaste = (event) => {
  event.preventDefault();
  const pasted = event.clipboardData
    .getData("text")
    .replace(/\D/g, "")
    .slice(0, 5);
  pasted.split("").forEach((digit, index) => {
    digits.value[index] = digit;
  });
  nextTick(() => inputRefs.value[Math.min(pasted.length, 4)]?.focus());
  if (pasted.length === 5) nextTick(pickup);
};

const pickup = async () => {
  const code = digits.value.join("");
  if (!/^\d{5}$/.test(code)) {
    toast.error(t("home.invalidCode"));
    return;
  }
  if (loading.value) return;
  loading.value = true;
  try {
    const response = await fetch(`/clip/${code}/resolve`, {
      method: "POST",
      headers: { Accept: "application/json" },
    });
    const data = await response.json().catch(() => ({}));
    if (!response.ok)
      throw new Error(
        response.status === 404
          ? t("home.notFound")
          : t("common.requestFailed", { status: response.status }),
      );
    pickupResult.value = data;
    const record = { ...data, retrievedAt: new Date().toISOString() };
    pickupHistory.value = [
      record,
      ...pickupHistory.value.filter((item) => item.code !== data.code),
    ].slice(0, 10);
    localStorage.setItem("pickupHistory", JSON.stringify(pickupHistory.value));
    resultOpen.value = true;
  } catch (error) {
    toast.error(
      error instanceof TypeError ? t("common.networkError") : error.message,
    );
  } finally {
    loading.value = false;
  }
};

const openHistoryRecord = (record) => {
  historyOpen.value = false;
  pickupResult.value = record;
  resultOpen.value = true;
};

const clearPickupHistory = () => {
  pickupHistory.value = [];
  localStorage.removeItem("pickupHistory");
  historyOpen.value = false;
  toast.success(t("home.pickupHistoryCleared"));
};
</script>

<style scoped>
.pickup-page {
  min-height: min(440px, calc(100vh - 210px));
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 2rem 0 4rem;
}

.pickup-heading {
  text-align: center;
}

.pickup-heading h1 {
  font-size: clamp(2rem, 5vw, 2.65rem);
  font-weight: 800;
  letter-spacing: -0.04em;
}

.pickup-heading p {
  margin-top: 0.6rem;
  color: hsl(var(--muted-foreground));
  font-size: 0.95rem;
}

.pickup-form {
  width: min(100%, 390px);
  margin-top: 2.6rem;
}

.pickup-code-grid {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 0.9rem;
}

.pickup-code-grid input {
  width: 100%;
  height: 80px;
  border: 1px solid hsl(var(--border) / 0.4);
  border-radius: 1rem;
  background: hsl(var(--card) / 0.3);
  color: hsl(var(--foreground));
  text-align: center;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 2rem;
  font-weight: 700;
  transition:
    border-color 160ms ease,
    background 160ms ease;
}

.pickup-code-grid input:focus {
  border-color: hsl(var(--foreground) / 0.65);
  background: hsl(var(--card) / 0.65);
}

.pickup-submit {
  width: 100%;
  height: 58px;
  margin-top: 2rem;
  border: 1px solid hsl(var(--border) / 0.45);
  border-radius: 1rem;
  background: hsl(var(--foreground) / 0.07);
  color: hsl(var(--muted-foreground));
  font-size: 1rem;
  font-weight: 700;
  transition:
    background 160ms ease,
    color 160ms ease,
    border-color 160ms ease;
}

.pickup-submit:not(:disabled):hover {
  border-color: hsl(var(--foreground) / 0.4);
  background: hsl(var(--foreground));
  color: hsl(var(--background));
}

.pickup-actions {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.85rem;
  margin-top: 0.85rem;
}

.pickup-secondary-action {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.55rem;
  height: 48px;
  border: 1px solid hsl(var(--border) / 0.28);
  border-radius: 0.9rem;
  color: hsl(var(--muted-foreground));
  font-size: 0.92rem;
  font-weight: 650;
  text-decoration: none;
  background: transparent;
  transition:
    border-color 160ms ease,
    background 160ms ease,
    color 160ms ease;
}

.pickup-secondary-action:hover {
  border-color: hsl(var(--foreground) / 0.45);
  background: hsl(var(--foreground) / 0.06);
  color: hsl(var(--foreground));
}

.pickup-room-action {
  grid-column: 1 / -1;
}

@media (max-width: 480px) {
  .pickup-code-grid {
    gap: 0.5rem;
  }
  .pickup-code-grid input {
    height: 68px;
    border-radius: 0.8rem;
    font-size: 1.65rem;
  }
}
</style>
