test(
  'libapm-models',
  executable(
    'libapm-models-test',
    'models-test.vala',
    dependencies: [libapm_dep],
  )
)

if get_option('integration_tests')
  test(
    'libapm-jobs',
    executable(
      'libapm-jobs-test',
      'jobs-test.vala',
      dependencies: [libapm_dep],
    ),
    timeout: 30,
  )

  test(
    'libapm-api',
    executable('libapm-api-test', 'api-test.vala', dependencies: [libapm_dep]),
    timeout: 120,
  )

  libapm_e2e = executable(
    'libapm-e2e-test',
    'e2e-test.vala',
    dependencies: [libapm_dep],
  )
  test('libapm-live', libapm_e2e, suite: 'libapm-live', timeout: 600)
endif
