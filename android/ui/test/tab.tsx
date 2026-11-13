// One home tab on its own, for tests: headless tests cannot press the system tab bar.
import { host, mount, View } from "nsh";
import { Search } from "../src/pages/Search";
import { Settings } from "../src/pages/Settings";

const tab = (host.params as { tab?: string } | null)?.tab;

mount(() => <View class="flex-1 bg-background">{tab === "settings" ? <Settings /> : <Search />}</View>);
