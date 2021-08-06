# encoding: utf-8
# copyright: 2018, The Authors
title 'Terraform Habitat Provisioner Tests'
control 'terraform-provisioner-habitat-1.0' do
  impact 0.7
  title 'Verify Terraform Provisioner Habitat bootstrapped correctly'

  # Main Habitat Services are running
  describe command("hab svc status") do
    its(:exit_status) { should eq 0 }
    its(:stderr) { should be_empty }
    its(:stdout) { should match /klm\/effortless(.*)up(.*)up(.*)effortless.default/ }
  end

  # Chef Habitat effortless.spec has correct channel (unstable)
  describe file("/hab/sup/default/specs/effortless.spec") do
    it { should exist }
    it { should be_file }
    it { should be_owned_by 'root' }
    it { should be_grouped_into 'root' }
    its('mode') { should cmp '0600' }
    its(:content) { should match /ident = "klm\/effortless"/ }
    its(:content) { should match /group = "default"/ }
    its(:content) { should match /bldr_url = "https:\/\/bldr.habitat.sh\/"/ }
    its(:content) { should match /channel = "unstable"/ }
    its(:content) { should match /topology = "standalone"/ }
    its(:content) { should match /update_strategy = "at-once"/ }
    its(:content) { should match /update_condition = "latest"/ }
  end

  # Chef client attributes.json is present and has valid content
  describe command("cat /hab/svc/effortless/config/attributes.json | jq -r '.klm'") do
    its(:exit_status) { should eq 0 }
    its(:stderr) { should be_empty }
    its(:stdout) { should_not match /null/ }
  end

  # Chef Habitat user.toml for klm/effortless is present and has valid content
  describe file("/hab/user/effortless/config/user.toml") do
    it { should exist }
    it { should be_file }
    it { should be_owned_by 'root' }
    it { should be_grouped_into 'root' }
    its('mode') { should cmp '0644' }
    its(:content) { should match /interval = 300/ }
    its(:content) { should match /[attributes.klm.debian]/ }
    its(:content) { should match /[attributes.klm.machine]/ }
    its(:content) { should match /[[attributes.klm.machine.network.interfaces]]/ }
  end

  # Supervisor Listener(s)
  %w( 9631 9638 ).each do |p|
    describe port.where { port == p.to_i && protocol =~ /tcp/ } do
      it { should be_listening }
    end
  end

  # Verify HTTP listener requires authentication
  describe command("curl --silent -X GET http://localhost:9631/butterfly -w '%{http_code}'") do
    its(:exit_status) { should eq 0 }
    its(:stderr) { should be_empty }
    its(:stdout) { should match /^401$/ }
  end

  # Butterfly API should have >= 5 && <= 9 members
  describe command("curl --silent -X GET http://localhost:9631/butterfly -H 'Authorization: Bearer ea7-beef' | jq -r '.member.members | to_entries | length'") do
    its(:exit_status) { should eq 0 }
    its(:stderr) { should be_empty }
    its(:stdout) { should match /^[5-9]$/ }
  end

  # Census API for effortless.default should have all 5 machines
  describe command("curl --silent -X GET http://localhost:9631/census -H 'Authorization: Bearer ea7-beef' | jq -r '.census_groups | .\"effortless.default\" | .population | to_entries | .[] | .value.sys.hostname' | sort") do
    its(:exit_status) { should eq 0 }
    its(:stderr) { should be_empty }
    its(:stdout) { should match /^linux/ }
    its(:stdout) { should match /win/i }
    its(:stdout) { should match /^sup-ring-1/ }
    its(:stdout) { should match /^sup-ring-2/ }
    its(:stdout) { should match /^sup-ring-3/ }
  end
end
