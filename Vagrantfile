# -*- mode: ruby -*-
# vi: set ft=ruby :

require 'etc'

Vagrant.require_version ">= 2.0.0", "< 3.0.0"

Vagrant.configure("2") do |config|
  config.vm.box = "bento/ubuntu-20.04"

  config.vm.provider "virtualbox" do |vb|
    vb.memory = "2000"
    vb.cpus = Etc.nprocessors
  end

  config.vm.synced_folder "~/.circleci", '/home/vagrant/.circleci'
  config.vm.provision "shell", path: "bin/provision"
end
