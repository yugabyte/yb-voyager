import os
import sys
import argparse

def valparser():
    parser = argparse.ArgumentParser(description='Process an argument.')
    parser.add_argument('--live_migration', type=str, help='An argument for checking the if live migration is enabled or not')
    parser.add_argument('--ff_enabled', type=str, help='An argument for checking the if fall-forward is enabled or not')
    parser.add_argument('--fb_enabled', type=str, help='An argument for checking the if fall-back is enabled or not')
    args = parser.parse_args()
    test_type_flags = {
        'live_migration': args.live_migration,
        'ff_enabled': args.ff_enabled,
        'fb_enabled': args.fb_enabled,
    }

    return test_type_flags
