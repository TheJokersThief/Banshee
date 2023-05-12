local dsl = import '_dsl.libsonnet';

local jobs = dsl.jobs;
local pipeline = dsl.pipeline;
local steps = dsl.steps;
local workflows = dsl.workflows;
local orbs = dsl.orbs;

local tag_filter = workflows.filter_tags(only=['/v.*/']) + workflows.filter_branches(ignore=['/.*/']);
local branches_filter = workflows.filter_branches(only=['/.*/']) + workflows.filter_tags(ignore=['/.*/']);

local homedir = '/home/circleci/banshee';
local gover = '1.20';


pipeline.new(
    orbs=orbs.new({ go: 'circleci/go@1.7.3', 'gh': 'circleci/github-cli@2.2.0' }),
    workflows=[
        workflows.new(
            'test',
            jobs=[
                workflows.job(
                    'test_and_lint',
                    executor={ name:'go/default', tag: gover },
                    filters=branches_filter,
                    working_directory=homedir,
                    steps=[
                        steps.checkout(),
                        'go/load-cache',
                        'go/mod-download',
                        { 'go/test': { 
                            covermode: "atomic",
                            failfast: true,
                            race: true,
                        }},
                        steps.run('go get github.com/golangci/golangci-lint/cmd/golangci-lint'),
                        steps.run('go run github.com/golangci/golangci-lint/cmd/golangci-lint run ./...'),
                    ],
                )
            ],
        ),


        workflows.new(
            'build-and-release',
            jobs=[
                workflows.job(
                    'build',
                    executor={ name:'go/default', tag: gover },
                    filters=tag_filter,
                    working_directory=homedir,
                    steps=[
                        steps.checkout(),
                        'go/load-cache',
                        'go/mod-download',
                        'go/save-cache',
                        steps.run("curl --proto '=https' --tlsv1.2 -sSf https://just.systems/install.sh | bash -s -- --to ./", name='Download just'),
                        steps.run('./just build_all ${CIRCLE_TAG}', name='Build binary for all platforms'),
                        steps.store_artifacts('/home/circleci/banshee/dist/'),
                        steps.persist_to_workspace(root=homedir, paths=['dist']),
                    ],
                ),

                workflows.job(
                    'release',
                    image='cimg/base:stable',
                    requires=['build'],
                    filters=tag_filter,
                    working_directory=homedir,
                    steps=[
                        steps.checkout(),
                        steps.attach_workspace(homedir),
                        { 'gh/setup': { version: '2.28.0' } },
                        steps.run('gh release create ${CIRCLE_TAG} --generate-notes --verify-tag', name='Create a new release'),
                        steps.run('gh release upload ${CIRCLE_TAG} /home/circleci/banshee/dist/bin/*', name='Create a new release'),
                    ],
                )
            ],
        ),
    ],
)
