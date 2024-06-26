Diagrams in this directory are auto-generated.
Any PR that makes changes in the `docs/workspace.dsl` file in this repo will
trigger a GitHub action that updates the diagrams.
This will generate a second PR that can be reviewed before merging with the first PR.

# Contributing

If you want to edit the `workspace.dsl` and see your changes reflected during
testing you have two options:

1. The [online editor](https://structurizr.com/dsl) from Structurizr
2. The [CLI](https://docs.structurizr.com/cli) from Structurizr

The diagrams are built according to the [C4 model](https://c4model.com/).
This allows architectural diagrams to be constructed at different levels of abstraction from a single `workspace.dsl`.

If contributing to the diagrams you should be familiar with the C4 Model.
Any change you make could potentially impact multiple diagrams.
