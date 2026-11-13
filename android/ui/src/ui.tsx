// Small pieces the pages share: the desktop/web header, covers, state views, chips and the toast.
import { Image, safeArea, signal, Text, View } from "nsh";
import { Button } from "nsh/base";
import { t } from "./i18n";
import { nav } from "./nav";

/** The page's header off the phones; Android shows the system's, from the page's `title`. */
export function Header(props: { title: string }) {
  return (
    <View class="flex-row items-center gap-3 border-b border-border p-3">
      <If when={nav.routes().length > 1}>
        <Button onPress={() => nav.pop()} class="active:opacity-60">
          <Text class="text-primary">{t().back}</Text>
        </Button>
      </If>
      <Text class="font-semibold text-foreground">{props.title}</Text>
    </View>
  );
}

/** The system header in the app's colours instead of the platform grey. */
export const headerClass = "bg-background text-foreground tint-primary";

/** A 3:4 cover; covers are HEIC, which Android decodes natively. */
export function Cover(props: { url?: string; class?: string }) {
  return (
    <View class={`aspect-[3/4] overflow-hidden rounded-(--radius) bg-muted ${props.class ?? ""}`}>
      <If when={props.url}>{(url) => <Image src={url()} class="size-full object-cover" />}</If>
    </View>
  );
}

export function Loading() {
  return (
    <View class="flex-1 items-center justify-center p-8">
      <Text class="text-muted-foreground">{t().loading}</Text>
    </View>
  );
}

export function Message(props: { text: string; action?: string; onAction?: () => void }) {
  return (
    <View class="flex-1 items-center justify-center gap-4 p-8">
      <Text class="text-center text-muted-foreground">{props.text}</Text>
      <If when={props.action}>
        {(label) => (
          <Button onPress={() => props.onAction?.()} class="rounded-(--radius) bg-primary px-5 py-2 active:opacity-80">
            <Text class="text-primary-foreground">{label()}</Text>
          </Button>
        )}
      </If>
    </View>
  );
}

export function Chip(props: { label: string; on: boolean; onPress: () => void }) {
  return (
    <Button
      onPress={() => props.onPress()}
      class="rounded-full px-3 py-1.5 active:opacity-70"
      classList={{ "bg-primary": props.on, "bg-muted": !props.on }}
    >
      <Text class={props.on ? "text-sm text-primary-foreground" : "text-sm text-foreground"}>{props.label}</Text>
    </Button>
  );
}

const toastText = signal("");
let toastTimer: ReturnType<typeof setTimeout> | undefined;

/** A short message over the app's bottom for 2.5 s. */
export function toast(text: string) {
  toastText.set(text);
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => toastText.set(""), 2500);
}

export function ToastHost() {
  return (
    <If when={toastText()}>
      <View class="absolute inset-x-6 items-center" style={{ bottom: safeArea().bottom + 96, pointerEvents: "none" }}>
        <View class="rounded-full bg-popover px-4 py-2 shadow-lg">
          <Text class="text-sm text-popover-foreground">{toastText()}</Text>
        </View>
      </View>
    </If>
  );
}
