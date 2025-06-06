# coredns-nomad-plugin

A plugin for CoreDNS to pull config from nomad server

I'm not completely sure what I'm doing here, but trying to put something together to bridge the gap
between services running, and being discoverable externally, but without chains of disjoint
services bucket-brigading back and forth.  ...so Ironically, I'm looking into a service that
explicitly chains?  Let's see if the joke is on me, but I'm hoping that this will be a bit more
cohesively stitched together.

I'm feeling a bit like Nomad has a niche of being simple and small -- simpler and smaller than k3s
-- but then needs a host of tools around it, adding complexity and RAM/CPU demands.  I'm trying to
better understand nomad this way.
