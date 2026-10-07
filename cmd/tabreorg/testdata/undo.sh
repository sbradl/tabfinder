#!/bin/sh
# Reverts the moves logged in <root>-moves-<stamp>.tsv
set -e
cd '<root>'
mkdir -p 'Amber Marsh' && mv -n 'Amber Marsh/Dawn of the Copper Giant/Dawn of the Copper Giant.gp3' 'Amber Marsh/amber_marsh_dawn_of_the_copper_giant.gp3'
mkdir -p 'Argyle Moth' && mv -n 'Argyle Moth/Gluttonous.gp3' 'Argyle Moth/argyle_moth_gluttonous.gp3'
mkdir -p 'Broken' && mv -n 'Broken/Garbled.gp5' 'Broken/garbled.gp5'
mkdir -p 'Cousins of Marrow' && mv -n 'Cousins of Marrow/Moonbreder/Quiet Morning.gp3' 'Cousins of Marrow/Quiet Morning.gp3'
mkdir -p 'Cousins of Marrow' && mv -n 'Cousins of Marrow/Moonbreder/Tidebloom.gp3' 'Cousins of Marrow/Tidebloom.gp3'
mkdir -p 'Die Äther' && mv -n 'Felix Ferien/Endlich Ferien/Weniger.gp3' 'Die Äther/Felix Ferien - Weniger.gp3'
mkdir -p 'Sodbury Lane/Agent Marmalade' && mv -n 'Sodbury Lane/Agent Marmalade/Umgezogen.gp3.zip' 'Sodbury Lane/Agent Marmalade/sodbury lane_umgezogen.gp3.zip'
mkdir -p 'Soilbed Quartet' && mv -n 'Soilbed Quartet/Glass Orchard/Brass Kettle (2).gp' 'Soilbed Quartet/Brass Kettle.gp'
mkdir -p '.' && mv -n 'Soilbed Quartet/Glass Orchard/Rust Parade.gp3' 'Soilbed Quartet - Rust Parade.gp3'
mkdir -p '.' && mv -n 'Mystery Tab.gp3' 'mystery_tab.gp3'
