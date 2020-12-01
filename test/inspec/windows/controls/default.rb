# encoding: utf-8
# copyright: 2018, The Authors
title 'Terraform Habitat Provisioner Tests (Windows)'
control 'terraform-provisioner-habitat-1.0' do
  impact 0.7
  title 'Verify Terraform Provisioner Habitat bootstrapped correctly for Windows'

  # Windows habitat service
  describe service('Habitat') do
    it { should be_installed }
    it { should be_running }
  end

  # Supervisor Listener(s)
  %w( 9631 9638 ).each do |p|
    describe port.where { port == p.to_i && protocol =~ /tcp/ } do
      it { should be_listening }
    end
  end

  # Chef Habitat effortless.spec has correct channel (unstable)
  describe file("C:/hab/sup/default/specs/effortless.spec") do
    it { should exist }
    it { should be_file }
    its(:content) { should match /ident = "klm\/effortless"/ }
    its(:content) { should match /group = "default"/ }
    its(:content) { should match /bldr_url = "https:\/\/bldr.habitat.sh\/"/ }
    its(:content) { should match /channel = "unstable"/ }
    its(:content) { should match /topology = "standalone"/ }
    its(:content) { should match /update_strategy = "at-once"/ }
    its(:content) { should match /update_condition = "latest"/ }
  end

  # Verify HTTP listener requires authentication
  describe command("(Invoke-WebRequest -Uri 'http://localhost:9631/butterfly' -UseBasicParsing).StatusCode") do
    its(:exit_status) { should eq 1 }
    its(:stderr) { should match /401/ }
    its(:stdout) { should be_empty }
  end

  # Butterfly API should have 4 members
  describe command("(Invoke-WebRequest -Headers @{'Authorization' = 'Bearer ea7-beef'} -Uri 'http://localhost:9631/butterfly' -UseBasicParsing).Content | hab pkg exec core/jq-static jq -r '.member.members | to_entries | length'") do
    its(:exit_status) { should eq 0 }
    its(:stderr) { should be_empty }
    its(:stdout) { should match /^5/ }
  end

  # Butterfly API should have all 5 members as alive
  describe command(%q{(Invoke-WebRequest -Headers @{'Authorization' = 'Bearer ea7-beef'} -Uri 'http://localhost:9631/butterfly' -UseBasicParsing).Content | hab pkg exec core/jq-static jq -r '.member.health | to_entries | .[] | select(.value!=\"Alive\") | .key'}) do
    its(:exit_status) { should eq 0 }
    its(:stderr) { should be_empty }
    its(:stdout) { should match /^$/ }
  end

  # Census API for effortless.default should have all 5 machines
  describe command(%q{(Invoke-WebRequest -Headers @{'Authorization' = 'Bearer ea7-beef'} -Uri 'http://localhost:9631/census' -UseBasicParsing).Content | hab pkg exec core/jq-static jq -r '.census_groups | .\"effortless.default\" | .population | to_entries | .[] | .value.sys.hostname'}) do
    its(:exit_status) { should eq 0 }
    its(:stderr) { should be_empty }
    its(:stdout) { should match /^linux-.*$/ }
    its(:stdout) { should match /^windows-.*$/i }
    its(:stdout) { should match /^sup-ring-1-.*$/ }
    its(:stdout) { should match /^sup-ring-2-.*$/ }
    its(:stdout) { should match /^sup-ring-3-.*$/ }
  end
end
