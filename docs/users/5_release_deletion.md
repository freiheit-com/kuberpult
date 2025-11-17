# Delete app forever

For deleting an application completely from kuberpult, it first needs to be
removed from all environments. This can be done incrementally or in one step.
After the app has been removed from all environments, it can be removed
forever. Both parts of this workflow are available in the UI.

## Steps
* Make sure that you first remove the app from your mono-repo and merge that change.
* Use the dot menu entry "remove app 'x' from environments" to remove the app
  from one or more environments
  * Note that an environment can still show up as available for removal, if the app has
    is not on the environment anymore. Thus, when using incremental removal
    from environments, the safest way of using this is to select all environments.
* Finally, use the dot menu entry "complete removing app 'x'" after having
  ensured the app was removed from all envs first.
  Note that this will remove the app **immediately**.
