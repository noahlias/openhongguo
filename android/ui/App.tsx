// The app shell: settings and history at start, the theme, the stack, and the home page's system tabs.
import { effect, mount, setColorScheme, signal, View } from "nsh";
import { NavPage, NavStack } from "nsh/base";
import { TabBar } from "nsh/mobile";
import { reloadHistory } from "./src/history";
import { t } from "./src/i18n";
import { nav } from "./src/nav";
import { Discover } from "./src/pages/Discover";
import { Library } from "./src/pages/Library";
import { Search } from "./src/pages/Search";
import { SeriesPage } from "./src/pages/Series";
import { Settings } from "./src/pages/Settings";
import { Theater } from "./src/pages/Theater";
import { loadSettings, settings } from "./src/settings";
import { Header, headerClass, toast, ToastHost } from "./src/ui";

const TABS = ["discover", "search", "library", "settings"] as const;

function Home() {
  const tab = signal(0);
  const titles = () => [t().discover, t().search, t().library, t().settings];
  return (
    <NavPage title={titles()[tab()]} header={<Header title={titles()[tab()]} />} headerClass={headerClass}>
      {/* Every tab stays mounted, so each keeps its scroll and state. */}
      <View class="flex-1 bg-background">
        <View class="flex-1" classList={{ hidden: tab() !== 0 }}>
          <Discover />
        </View>
        <View class="flex-1" classList={{ hidden: tab() !== 1 }}>
          <Search />
        </View>
        <View class="flex-1" classList={{ hidden: tab() !== 2 }}>
          <Library />
        </View>
        <View class="flex-1" classList={{ hidden: tab() !== 3 }}>
          <Settings />
        </View>
      </View>
      <TabBar
        items={TABS.map((name, i) => ({ title: titles()[i], icon: `tab_${name}` }))}
        selectedIndex={tab()}
        onChange={({ index }) => tab.set(index)}
      />
    </NavPage>
  );
}

function App() {
  effect(() => {
    const theme = settings().theme;
    setColorScheme(theme === "auto" ? "system" : theme === "light" ? "light" : "dark");
  });
  return (
    <View class="flex-1 bg-background">
      <NavStack state={nav} class="flex-1">
        {(route) => {
          switch (route.name) {
            case "home":
              return <Home />;
            case "series":
              return <SeriesPage id={route.id} />;
            case "theater":
              return <Theater request={route.request} />;
          }
        }}
      </NavStack>
      <ToastHost />
    </View>
  );
}

loadSettings().catch((e) => toast(String(e)));
reloadHistory().catch((e) => toast(String(e)));
mount(App);
