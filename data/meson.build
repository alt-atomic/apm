conf = configuration_data()
conf.set('SERVICE_ID', SERVICE_ID)
conf.set('USER', 'root')
conf.set('BINDIR', get_option('prefix') / get_option('bindir'))
conf.set('NAME', meson.project_name())

configure_file(
  input: SERVICE_ID + '.policy.in',
  output: SERVICE_ID + '.policy',
  configuration: conf,
  install: true,
  install_dir: get_option('datadir') / 'polkit-1' / 'actions'
)

configure_file(
  input: SERVICE_ID + '.conf.in',
  output: SERVICE_ID + '.conf',
  configuration: conf,
  install: true,
  install_dir: get_option('sysconfdir') / 'dbus-1' / 'system.d'
)

configure_file(
  input: SERVICE_ID + '.service.in',
  output: SERVICE_ID + '.service',
  configuration: conf,
  install: true,
  install_dir: get_option('datadir') / 'dbus-1' / 'system-services'
)

configure_file(
  input: SERVICE_ID + '.User.service.in',
  output: SERVICE_ID + '.User.service',
  configuration: conf,
  install: true,
  install_dir: get_option('datadir') / 'dbus-1' / 'services'
)

systemd = dependency('systemd')
systemd_system_unit_dir = systemd.get_variable(pkgconfig: 'systemdsystemunitdir')

systemd_tmpfiles_dir = systemd.get_variable(pkgconfig: 'tmpfiles_dir')

tmpfiles_conf = configuration_data()
tmpfiles_conf.set('VARAPMDIR', varapmdir)
tmpfiles_conf.set('CONFAPMDIR', confapmdir)
tmpfiles_conf.set('VARCACHEAPMDIR', varcacheapmdir)

configure_file(
  input: meson.project_name() + '.tmpfiles.in',
  output: meson.project_name() + '.conf',
  configuration: tmpfiles_conf,
  install: true,
  install_dir: systemd_tmpfiles_dir
)

configure_file(
  input: meson.project_name() + '.service.in',
  output: meson.project_name() + '.service',
  configuration: conf,
  install: true,
  install_dir: systemd_system_unit_dir
)

# --- Shell completions ---
bash_comp_dep = dependency('bash-completion', required: false)
bash_completion_dir = bash_comp_dep.found() ? bash_comp_dep.get_variable(pkgconfig: 'completionsdir') : (get_option('datadir') / 'bash-completion' / 'completions')
install_data(
  'completion/bash/apm',
  install_dir: bash_completion_dir,
  rename: 'apm'
)

fish_completion_dir = get_option('datadir') / 'fish' / 'vendor_completions.d'
install_data(
  'completion/fish/apm.fish',
  install_dir: fish_completion_dir
)

zsh_completion_dir = get_option('datadir') / 'zsh' / 'site-functions'
install_data(
  'completion/zsh/_apm',
  install_dir: zsh_completion_dir
)

# --- APT bridge configuration ---
apt_conf_dir = get_option('sysconfdir') / 'apt' / 'apt.conf.d'
apt_scripts_dir = get_option('datadir') / 'apt' / 'scripts'

install_data(
  'apt-bridge/99-apm-update.conf',
  install_dir: apt_conf_dir
)

install_data(
  'apt-bridge/update-apm.lua',
  install_dir: apt_scripts_dir
)
