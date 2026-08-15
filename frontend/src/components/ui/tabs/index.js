import { cva } from "class-variance-authority";

export { default as Tabs } from "./Tabs.vue";
export { default as TabsContent } from "./TabsContent.vue";
export { default as TabsList } from "./TabsList.vue";
export { default as TabsTrigger } from "./TabsTrigger.vue";

export const tabsListVariants = cva(
  "inline-flex box-border h-10 w-full items-center justify-center gap-1 p-1.5 text-muted-foreground",
  {
    variants: {
      variant: {
        default: "bg-transparent border border-border",
        line: "gap-1 bg-transparent",
      },
    },
    defaultVariants: {
      variant: "default",
    },
  },
);
