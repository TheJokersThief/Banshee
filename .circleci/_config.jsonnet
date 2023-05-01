local dsl = import '_dsl.libsonnet';

local jobs = dsl.jobs;
local pipeline = dsl.pipeline;
local steps = dsl.steps;
local workflows = dsl.workflows;
local orbs = dsl.orbs;

local tag_filter = workflows.filter_tags(only=['/v.*/']);


pipeline.new(
    orbs=orbs.new({ go: 'circleci/go@1.7.3', 'gh': 'circleci/github-cli@2.2.0' }),
    workflows=[
        workflows.new(
            'build-and-release',
            jobs=[
                workflows.job(
                    'build',
                    executor={ name:'go/default', tag: '1.19' },
                    filters=tag_filter,
                    working_directory='/banshee',
                    steps=[
                        steps.checkout(),
                        'go/load-cache',
                        'go/mod-download',
                        'go/save-cache',
                        steps.run("curl --proto '=https' --tlsv1.2 -sSf https://just.systems/install.sh | bash -s -- --to ./just", name='Download just'),
                        steps.run('bash ./just build_all ${CIRCLE_TAG}', name='Build binary for all platforms'),
                        steps.store_artifacts('/banshee/dist/'),
                        steps.persist_to_workspace(root='/banshee', paths=['dist']),
                    ],
                ),

                workflows.job(
                    'release',
                    image='cimg/base:stable',
                    requires=['build'],
                    filters=tag_filter,
                    working_directory='/banshee',
                    steps=[
                        steps.attach_workspace('/banshee/dist'),
                        'gh/setup',
                        steps.run('gh release create ${CIRCLE_TAG}, --generate-notes --verify-tag', name='Create a new release')
                    ],
                )
            ],
        ),
    ],
)
