# **nexSign mini (NSM) Licensing and Contribution Guidelines**

## Our Philosophy: Open, Resilient, and Sustainable

The mission of the nexSign mini (NSM) project is to build a robust, decentralized discovery and management fabric for digital signage. We aim to create a solution that is zero-configuration, resilient, and lightweight. To achieve this goal and ensure that NSM can be maintained as a high-quality tool for the community, we have adopted a model that is both open to innovation and commercially sustainable.

This document outlines the licensing for the NSM software and the guidelines for contributing to it. Our goal is to be transparent, fair, and to create a virtuous cycle where community contributions and commercial success mutually reinforce each other.

### **The NSM Dual-Licensing Model**

The nexSign mini (NSM) software is **dual-licensed**. This strategy provides a free, open-source option for the community while creating a revenue stream from commercial entities to fund the project's long-term development and stewardship.

#### **NSM Community Edition (GPLv3): The Community License**

The core NSM software is licensed under the **GNU General Public License v3 (GPLv3)**.

* **What it means:** The GPLv3 is a "copyleft" license. It grants anyone the freedom to use, study, modify, and distribute the software. In return, it requires that any derivative works or applications that link to the GPLv3 code must also be made available under the same or a compatible open-source license.
* **Why we chose it:** The GPLv3 ensures that NSM and its core protocols will remain free and open-source *forever*. It prevents the core from being captured and closed off by a single entity, guaranteeing a level playing field for the entire community.

#### **NSM Commercial License: The Sustainability License**

We recognize that the copyleft requirements of the GPLv3 are not compatible with all business models, particularly for device manufacturers who need to ship proprietary, locked-down firmware or for enterprises that wish to integrate NSM into their closed-source products.

For these use cases, NDX Pty Ltd offers a **commercial license**.

* **What it is:** The commercial license is effectively a "GPL exception." It allows a licensee to embed and distribute NSM software within their proprietary products without being bound by the GPLv3's source-sharing obligations.
* **Why it exists:** This is the financial engine that makes the entire project sustainable. Revenue generated from commercial licensing is reinvested directly into the project to:
    * Fund a core team of full-time developers and maintainers.
    * Ensure long-term security audits and maintenance.
    * Support the operational costs of the project.

## Contributing to NSM

We believe the best software is built in collaboration with a diverse community. Your contributions, from documentation fixes to major feature development, are not only welcome—they are essential to our mission.

### **An Invitation to Build With Us**

When you contribute to NSM, you are doing more than just submitting code; you are joining a mission to build a smarter, more resilient digital signage ecosystem. We view our contributors as vital partners in this journey.

#### **The Contributor License Agreement (CLA) Requirement**

To enable our dual-licensing model, which is essential for the project's long-term health, we require all contributors to sign a Contributor License Agreement (CLA) before their first pull request can be merged.

* **Why is a CLA necessary?** The CLA is a legal document that gives NDX Pty Ltd (as the project's steward) the right to re-license your contribution. It is the mechanism that allows us to offer your code under both the GPLv3 and our commercial license. Without this agreement, we would be legally unable to include your contribution in our commercially licensed products, which would fragment the codebase and undermine the project's sustainability model.
* **What it means for you, the contributor:**
    * **You retain full ownership** and copyright of your original contribution.
    * You grant the NSM project a broad, non-exclusive license to use, modify, and distribute your work as part of the NSM software stack.
    * You are confirming that you have the right to make this contribution.
* **How it ensures the project's success:** By signing the CLA, you are directly enabling the business model that funds the project's growth. The commercial licensing of your contribution is what allows us to pay engineers to maintain the codebase, fix bugs, and provide the stable foundation upon which the entire community builds. Your contribution becomes part of a sustainable ecosystem that benefits everyone.

### **The Contribution Process**

1.  **Discuss:** For any significant change, please open an issue or start a discussion to outline your idea. This ensures alignment with the project's roadmap before significant work is undertaken.
2.  **Fork & Branch:** Fork the repository and create a new branch for your feature or bugfix.
3.  **Sign the CLA:** You will be prompted to sign the CLA automatically via a pull request check on your first contribution.
4.  **Submit a Pull Request:** Once your work is complete, submit a pull request for review by the maintainers.

For major architectural changes, we may use the formal **NDX Improvement Proposal (NIP)** process, which is managed by the Technical Steering Committee of the NDX Foundation.