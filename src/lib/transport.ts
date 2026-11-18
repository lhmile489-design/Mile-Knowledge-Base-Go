// transport is the only module that talks to the generated bindings.
// Components and stores never import ./mygo directly.
import { AppService } from "../mygo";

export async function ping(): Promise<string> {
  return AppService.ping();
}
