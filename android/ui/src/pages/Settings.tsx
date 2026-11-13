// Settings: language, theme, quality, autoplay, and a new anonymous device (desktop/src/ui/pages/settings.rs).
import { Scroll, Text, View, type JSX } from "nsh";
import { Button, Switch } from "nsh/base";
import { isZh, t } from "../i18n";
import { call, describe } from "../sdk";
import { settings, updateSettings, type Settings as Prefs } from "../settings";
import { Chip, toast } from "../ui";

export const QUALITIES = ["best", "1080p", "720p", "540p", "worst"];

const save = (change: Partial<Prefs>) => updateSettings(change).catch((e) => toast(describe(e, isZh())));

function Row(props: { label: string; children: JSX.Element }) {
  return (
    <View class="gap-2 border-b border-border px-4 py-3">
      <Text class="text-sm text-muted-foreground">{props.label}</Text>
      {props.children}
    </View>
  );
}

function Options(props: { items: [string, string][]; value: string; onSelect: (v: string) => void }) {
  return (
    <View class="flex-row flex-wrap gap-2">
      <Each of={props.items}>{(item) => <Chip label={item()[1]} on={props.value === item()[0]} onPress={() => props.onSelect(item()[0])} />}</Each>
    </View>
  );
}

export function Settings() {
  const qualityLabel = (q: string) => (q === "best" ? t().qualityBest : q === "worst" ? t().qualityWorst : q);
  const resetDevice = () =>
    call("resetDevice").then(
      () => toast(t().deviceReset),
      (e) => toast(describe(e, isZh())),
    );
  return (
    <Scroll class="flex-1" contentClass="pb-[calc(env(safe-area-inset-bottom)+16px)]">
      <Row label={t().setLanguage}>
        <Options
          items={[
            ["zh", "中文"],
            ["en", "English"],
          ]}
          value={settings().language}
          onSelect={(language) => save({ language })}
        />
      </Row>
      <Row label={t().setTheme}>
        <Options
          items={[
            ["auto", t().themeAuto],
            ["dark", t().themeDark],
            ["light", t().themeLight],
          ]}
          value={settings().theme}
          onSelect={(theme) => save({ theme })}
        />
      </Row>
      <Row label={t().setQuality}>
        <Options items={QUALITIES.map((q) => [q, qualityLabel(q)] as [string, string])} value={settings().quality} onSelect={(quality) => save({ quality })} />
      </Row>
      <View class="flex-row items-center justify-between border-b border-border px-4 py-3">
        <Text class="text-foreground">{t().setAutoplay}</Text>
        <Switch
          checked={settings().autoplay}
          onChange={({ checked }) => save({ autoplay: checked })}
          accessibilityLabel={t().setAutoplay}
          trackClass="w-11 rounded-full bg-muted p-0.5 transition-colors checked:bg-primary"
          thumbClass="size-5 rounded-full bg-background transition-transform checked:translate-x-5"
        />
      </View>
      <View class="gap-2 px-4 py-4">
        <Button onPress={resetDevice} class="self-start rounded-(--radius) border border-border px-4 py-2 active:bg-muted">
          <Text class="text-foreground">{t().setResetDevice}</Text>
        </Button>
        <Text class="text-xs text-muted-foreground">{t().setResetHint}</Text>
      </View>
    </Scroll>
  );
}
