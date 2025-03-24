import { OperatingSystem } from "@/lib/types";
import { MessagesSquare, Scroll } from "lucide-react";
import React from "react";
import { FaDiscord, FaGithub } from "react-icons/fa";

export const basePath = process.env.BASE_PATH;

const isMobile = typeof window !== "undefined" && window.innerWidth < 640;

export const navbarLinks = [
	{
		href: `https://github.com/bketelsen/${basePath}`,
		event: "Github",
		icon: <FaGithub className="h-4 w-4" />,
		text: "Github",
	},
	// {
	// 	href: `https://discord.gg/2wvnMDgdnU`,
	// 	event: "Discord",
	// 	icon: <FaDiscord className="h-4 w-4" />,
	// 	text: "Discord",
	// },
	{
		href: `https://github.com/bketelsen/${basePath}/blob/main/CHANGELOG.md`,
		event: "Change Log",
		icon: <Scroll className="h-4 w-4" />,
		text: "Change Log",
    mobileHidden: true,
	},
	{
		href: `https://github.com/bketelsen/${basePath}/discussions`,
		event: "Discussions",
		icon: <MessagesSquare className="h-4 w-4" />,
		text: "Discussions",
    mobileHidden: true,
	},
].filter(Boolean) as {
	href: string;
	event: string;
	icon: React.ReactNode;
	text: string;
	mobileHidden?: boolean;
}[];

export const mostPopularScripts = ["post-pve-install", "docker", "homeassistant"];

export const analytics = {
  url: "a.ketelsen.cloud",
  token: "00c7925b-718c-498f-843f-e744c610dccb",
};

export const AlertColors = {
  warning: "border-red-500/25 bg-destructive/25",
  info: "border-cyan-500/25 bg-cyan-50 dark:border-cyan-900 dark:bg-cyan-900/25",
};

export const OperatingSystems: OperatingSystem[] = [
  {
    name: "Debian",
    versions: [
      { name: "11", slug: "bullseye" },
      { name: "12", slug: "bookworm" },
    ],
  },
  {
    name: "Ubuntu",
    versions: [
      { name: "22.04", slug: "jammy" },
      { name: "24.04", slug: "noble" },
    ],
  },
];
