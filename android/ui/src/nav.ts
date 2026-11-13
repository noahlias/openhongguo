// The navigation stack, created once; pages import it to navigate.
import { createNavStack } from "nsh/base";
import type { PlayRequest } from "./session";

export type Route = { name: "home" } | { name: "series"; id: string } | { name: "theater"; request: PlayRequest };

export const nav = createNavStack<Route>([{ name: "home" }]);
