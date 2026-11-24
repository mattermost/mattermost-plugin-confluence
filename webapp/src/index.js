import '@mattermost/compass-ui/styles';

import manifest from './manifest';

import Hooks from './hooks';
import reducer from './reducers';

import SubscriptionModal from './components/subscription_modal';

//
// Define the plugin class that will register
// our plugin components.
//
class PluginClass {
    initialize(registry, store) {
        registry.registerReducer(reducer);
        registry.registerRootComponent(SubscriptionModal);
        const hooks = new Hooks(store);
        registry.registerSlashCommandWillBePostedHook(hooks.slashCommandWillBePostedHook);
    }
}

//
// To register your plugin you must expose it on window.
//
window.registerPlugin(manifest.id, new PluginClass());
